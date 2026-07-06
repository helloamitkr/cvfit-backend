package handlers

import (
	"encoding/json"
	"net/http"

	apperrors "github.com/helloamitkr/cvfit-tools/errors"
)

func (d *Deps) HandleCreateResumeRequest(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromCtx(r.Context())
	var body struct {
		Name       string `json:"name"`
		Email      string `json:"email"`
		ResumeLink string `json:"resume_link"`
		WantsCall  bool   `json:"wants_call"`
		Notes      string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, apperrors.BadRequest("invalid JSON body"))
		return
	}
	if body.Name == "" || body.Email == "" {
		writeError(w, apperrors.BadRequest("name and email are required"))
		return
	}
	resp, appErr := d.Svc.CreateResumeRequest(
		r.Context(), userID,
		body.Name, body.Email, body.ResumeLink, body.Notes, body.WantsCall,
	)
	if appErr != nil {
		writeError(w, appErr)
		return
	}
	writeSuccess(w, http.StatusCreated, resp)
}

func (d *Deps) HandleVerifyRequestPayment(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RazorpayOrderID string `json:"order_id"`
		PaymentID       string `json:"payment_id"`
		Signature       string `json:"signature"`
		RequestID       string `json:"request_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, apperrors.BadRequest("invalid JSON body"))
		return
	}
	if appErr := d.Svc.VerifyRequestPayment(
		r.Context(),
		body.RazorpayOrderID, body.PaymentID, body.Signature, body.RequestID,
	); appErr != nil {
		writeError(w, appErr)
		return
	}
	writeSuccess(w, http.StatusOK, map[string]bool{"paid": true})
}

func (d *Deps) HandleAdminRequests(w http.ResponseWriter, r *http.Request) {
	requests, err := d.Svc.ListRequests(r.Context())
	if err != nil {
		writeError(w, apperrors.InternalServerWrap("failed to list requests", err))
		return
	}
	writeSuccess(w, http.StatusOK, requests)
}
