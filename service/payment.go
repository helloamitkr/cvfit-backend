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
	"github.com/helloamitkr/cvfit-backend/model"
	apperrors "github.com/helloamitkr/cvfit-tools/errors"
)

// OrderStatusFailed marks an order as failed (called when Razorpay modal is dismissed).
func (s *Service) OrderStatusFailed(ctx context.Context, orderID string) {
	key := map[string]types.AttributeValue{
		"id": &types.AttributeValueMemberS{Value: orderID},
	}
	var o model.Order
	if err := s.Dynamo.GetItem(ctx, s.OrdersTableName, key, &o); err != nil || o.ID == "" {
		return
	}
	if o.Status != "pending" {
		return
	}
	o.Status = "failed"
	o.UpdatedAt = time.Now().UTC()
	// best-effort — if this fails the order stays "pending" and expires naturally
	_ = s.Dynamo.PutItem(ctx, s.OrdersTableName, &o)
}

// OrderResponse is returned to the client to open the Razorpay checkout.
type OrderResponse struct {
	OrderID  string `json:"order_id"`
	Amount   int    `json:"amount"`
	Currency string `json:"currency"`
	KeyID    string `json:"key_id"`
	ResumeID string `json:"resume_id"`
}

// CreatePaymentOrder creates a Razorpay order and records it as pending in the orders table.
func (s *Service) CreatePaymentOrder(ctx context.Context, userID, resumeID string) (*OrderResponse, *apperrors.AppError) {
	userEmail := ""
	if u, err := s.GetByID(ctx, userID); err == nil && u != nil {
		userEmail = u.Email
	}

	payload := map[string]interface{}{
		"amount":   s.PaymentAmountINR,
		"currency": "INR",
		"receipt":  "rcpt_" + userID[:8],
		"notes": map[string]string{
			"user_id":   userID,
			"resume_id": resumeID,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, apperrors.InternalServerWrap("failed to build order payload", err)
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

	orderID, _ := result["id"].(string)
	now := time.Now().UTC()
	if err := s.Dynamo.PutItem(ctx, s.OrdersTableName, &model.Order{
		ID:        orderID,
		UserID:    userID,
		UserEmail: userEmail,
		ResumeID:  resumeID,
		Amount:    s.PaymentAmountINR,
		Status:    "pending",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		return nil, apperrors.InternalServerWrap("failed to record order", err)
	}

	return &OrderResponse{
		OrderID:  orderID,
		Amount:   s.PaymentAmountINR,
		Currency: "INR",
		KeyID:    s.RazorpayKeyID,
		ResumeID: resumeID,
	}, nil
}

// VerifyAndActivate validates the Razorpay signature and marks the order as successful.
// S3URL and IsEmailSent are set later by the receipt pipeline.
func (s *Service) VerifyAndActivate(ctx context.Context, userID, resumeID, orderID, paymentID, signature string) *apperrors.AppError {
	mac := hmac.New(sha256.New, []byte(s.RazorpayKeySecret))
	mac.Write([]byte(orderID + "|" + paymentID))
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return apperrors.New(http.StatusBadRequest, "invalid payment signature")
	}

	key := map[string]types.AttributeValue{
		"id": &types.AttributeValueMemberS{Value: orderID},
	}
	var o model.Order
	if err := s.Dynamo.GetItem(ctx, s.OrdersTableName, key, &o); err != nil || o.ID == "" {
		return apperrors.NotFound("order not found")
	}

	now := time.Now().UTC()
	o.Status = "success"
	o.PaymentID = paymentID
	o.PaidAt = &now
	o.UpdatedAt = now
	if err := s.Dynamo.PutItem(ctx, s.OrdersTableName, &o); err != nil {
		return apperrors.InternalServerWrap("failed to update order", err)
	}

	// The ₹10 purchase is a bundle: it also lifts today's free AI limit.
	s.GrantAIUnlock(ctx, userID)

	return nil
}

// HasRecentPayment returns true if the given order is a successful payment for this user.
// orderID must be the Razorpay order ID returned from the payment flow.
// Using GetItem ensures strongly consistent reads — no eventual consistency lag.
func (s *Service) HasRecentPayment(ctx context.Context, userID, orderID string) bool {
	if userID == "" || orderID == "" {
		return false
	}
	key := map[string]types.AttributeValue{
		"id": &types.AttributeValueMemberS{Value: orderID},
	}
	var o model.Order
	if err := s.Dynamo.GetItem(ctx, s.OrdersTableName, key, &o); err != nil {
		return false
	}
	return o.UserID == userID && o.Status == "success"
}
