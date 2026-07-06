package handlers

import (
	"encoding/json"
	"net/http"

	apperrors "github.com/helloamitkr/cvfit-tools/errors"
)

// HandleCreateOrder creates a Razorpay order for a specific resume.
// POST /api/payment/create-order
// Body: { "resume_id": "..." }
func (d *Deps) HandleCreateOrder(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromCtx(r.Context())
	if userID == "" {
		writeError(w, apperrors.New(http.StatusUnauthorized, "authentication required"))
		return
	}

	var body struct {
		ResumeID string `json:"resume_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ResumeID == "" {
		writeError(w, apperrors.BadRequest("resume_id is required"))
		return
	}

	order, appErr := d.Svc.CreatePaymentOrder(r.Context(), userID, body.ResumeID)
	if appErr != nil {
		writeError(w, appErr)
		return
	}
	writeSuccess(w, http.StatusOK, order)
}

// HandleVerifyPayment validates the Razorpay signature and stores the payment.
// POST /api/payment/verify
// Body: { "resume_id": "...", "order_id": "...", "payment_id": "...", "signature": "..." }
func (d *Deps) HandleVerifyPayment(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromCtx(r.Context())
	if userID == "" {
		writeError(w, apperrors.New(http.StatusUnauthorized, "authentication required"))
		return
	}

	var body struct {
		ResumeID  string `json:"resume_id"`
		OrderID   string `json:"order_id"`
		PaymentID string `json:"payment_id"`
		Signature string `json:"signature"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, apperrors.BadRequest("invalid JSON body"))
		return
	}
	if body.ResumeID == "" || body.OrderID == "" || body.PaymentID == "" || body.Signature == "" {
		writeError(w, apperrors.BadRequest("resume_id, order_id, payment_id and signature are required"))
		return
	}

	if appErr := d.Svc.VerifyAndActivate(r.Context(), userID, body.ResumeID, body.OrderID, body.PaymentID, body.Signature); appErr != nil {
		writeError(w, appErr)
		return
	}
	writeSuccess(w, http.StatusOK, map[string]bool{"activated": true})
}

// HandlePaymentStatus checks if a specific order is paid.
// GET /api/payment/status?order_id=...
func (d *Deps) HandlePaymentStatus(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromCtx(r.Context())
	orderID := r.URL.Query().Get("order_id")
	hasPaid := d.Svc.HasRecentPayment(r.Context(), userID, orderID)
	writeSuccess(w, http.StatusOK, map[string]interface{}{
		"has_paid": hasPaid,
	})
}
