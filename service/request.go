package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
	"github.com/helloamitkr/cvfit-backend/model"
	apperrors "github.com/helloamitkr/cvfit-tools/errors"
)

const RequestAmountPaise = 20000 // ₹200

// RequestOrderResponse is returned to the client to open the Razorpay checkout.
type RequestOrderResponse struct {
	RazorpayOrderID string `json:"order_id"`
	RequestID       string `json:"request_id"`
	Amount          int    `json:"amount"`
	Currency        string `json:"currency"`
	KeyID           string `json:"key_id"`
}

// CreateResumeRequest saves a pending request and creates a Razorpay order for ₹500.
func (s *Service) CreateResumeRequest(
	ctx context.Context,
	userID, name, email, resumeLink, notes string,
	wantsCall bool,
) (*RequestOrderResponse, *apperrors.AppError) {
	payload := map[string]interface{}{
		"amount":   RequestAmountPaise,
		"currency": "INR",
		"receipt":  "req_" + userID[:min(8, len(userID))],
		"notes": map[string]string{
			"user_id": userID,
			"type":    "create_request",
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, apperrors.InternalServerWrap("failed to build request payload", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.razorpay.com/v1/orders", bytes.NewReader(body))
	if err != nil {
		return nil, apperrors.InternalServerWrap("failed to build razorpay request", err)
	}
	req.SetBasicAuth(s.RazorpayKeyID, s.RazorpayKeySecret)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, apperrors.InternalServerWrap("razorpay request failed", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperrors.InternalServerWrap("failed to read razorpay response", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(rawBody, &result); err != nil {
		return nil, apperrors.InternalServerWrap("failed to parse razorpay response", err)
	}
	if resp.StatusCode != http.StatusOK {
		msg, _ := result["error"].(map[string]interface{})
		desc := fmt.Sprintf("razorpay error: %v — %v", msg["code"], msg["description"])
		return nil, apperrors.New(http.StatusBadGateway, desc)
	}

	razorpayOrderID, _ := result["id"].(string)
	now := time.Now().UTC()
	requestID := uuid.New().String()

	record := &model.ResumeRequest{
		ID:         requestID,
		UserID:     userID,
		Name:       name,
		Email:      email,
		ResumeLink: resumeLink,
		WantsCall:  wantsCall,
		Notes:      notes,
		Amount:     RequestAmountPaise,
		Status:     "pending",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s.Dynamo.PutItem(ctx, s.RequestsTableName, record); err != nil {
		return nil, apperrors.InternalServerWrap("failed to save request", err)
	}

	// Map Razorpay order ID → request ID so verify can look it up
	mapping := &requestMapping{
		ID:        razorpayOrderID,
		RequestID: requestID,
		CreatedAt: now,
	}
	if err := s.Dynamo.PutItem(ctx, s.RequestsTableName, mapping); err != nil {
		return nil, apperrors.InternalServerWrap("failed to save order mapping", err)
	}

	return &RequestOrderResponse{
		RazorpayOrderID: razorpayOrderID,
		RequestID:       requestID,
		Amount:          RequestAmountPaise,
		Currency:        "INR",
		KeyID:           s.RazorpayKeyID,
	}, nil
}

// requestMapping stores the razorpay_order_id → request_id link in the same table.
type requestMapping struct {
	ID        string    `json:"id" dynamodbav:"id"`
	RequestID string    `json:"request_id" dynamodbav:"request_id"`
	CreatedAt time.Time `json:"created_at" dynamodbav:"created_at"`
}

// VerifyRequestPayment validates the Razorpay signature and marks the request as paid.
func (s *Service) VerifyRequestPayment(
	ctx context.Context,
	razorpayOrderID, paymentID, signature, requestID string,
) *apperrors.AppError {
	mac := hmac.New(sha256.New, []byte(s.RazorpayKeySecret))
	mac.Write([]byte(razorpayOrderID + "|" + paymentID))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return apperrors.New(http.StatusBadRequest, "invalid payment signature")
	}

	key := map[string]types.AttributeValue{
		"id": &types.AttributeValueMemberS{Value: requestID},
	}
	var r model.ResumeRequest
	if err := s.Dynamo.GetItem(ctx, s.RequestsTableName, key, &r); err != nil || r.ID == "" {
		return apperrors.NotFound("request not found")
	}

	now := time.Now().UTC()
	r.Status = "paid"
	r.PaymentID = paymentID
	r.PaidAt = &now
	r.UpdatedAt = now
	if err := s.Dynamo.PutItem(ctx, s.RequestsTableName, &r); err != nil {
		return apperrors.InternalServerWrap("failed to update request", err)
	}
	return nil
}

// ListRequests returns all resume creation requests (admin only).
func (s *Service) ListRequests(ctx context.Context) ([]model.ResumeRequest, error) {
	var all []model.ResumeRequest
	if err := s.Dynamo.Scan(ctx, s.RequestsTableName, &all); err != nil {
		return nil, err
	}
	// Filter out the mapping records (they have no Name field)
	requests := []model.ResumeRequest{}
	for _, r := range all {
		if r.Name != "" {
			requests = append(requests, r)
		}
	}
	return requests, nil
}
