package handlers

import (
	"encoding/json"
	"io"
	"net/http"

	apperrors "github.com/helloamitkr/cvfit-tools/errors"
	"github.com/helloamitkr/cvfit-tools/resume"
)

func (d *Deps) HandleRenderResume(w http.ResponseWriter, r *http.Request) {
	var rv resume.Resume
	if err := json.NewDecoder(r.Body).Decode(&rv); err != nil {
		writeError(w, apperrors.BadRequest("invalid JSON body"))
		return
	}
	html, appErr := d.Svc.RenderResume(r.Context(), &rv)
	if appErr != nil {
		writeError(w, appErr)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, html)
}

func (d *Deps) HandleGeneratePDF(w http.ResponseWriter, r *http.Request) {
	var body struct {
		resume.Resume
		OrderID   string `json:"order_id"`  // set by frontend after Razorpay payment
		SendEmail bool   `json:"email"`     // if true and user is authenticated + paid, also email the PDF
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, apperrors.BadRequest("invalid JSON body"))
		return
	}

	userID := UserIDFromCtx(r.Context())
	// Admins always get the clean (unwatermarked) PDF without paying.
	hasPaid := d.Svc.IsAdmin(r.Context(), userID) || d.Svc.HasRecentPayment(r.Context(), userID, body.OrderID)

	rv := body.Resume
	pdfBytes, appErr := d.Svc.GeneratePDF(r.Context(), &rv, hasPaid)
	if appErr != nil {
		writeError(w, appErr)
		return
	}

	filename := "resume_preview.pdf"
	if hasPaid && rv.Personal.FirstName != "" {
		filename = rv.Personal.FirstName + "_" + rv.Personal.LastName + "_resume.pdf"
	}

	// Email PDF to the authenticated user in a background goroutine (non-blocking).
	if hasPaid && body.SendEmail && userID != "" {
		pdfCopy := make([]byte, len(pdfBytes))
		copy(pdfCopy, pdfBytes)
		fn := filename
		go d.Svc.EmailResumePDF(r.Context(), userID, fn, pdfCopy)
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdfBytes)
}
