package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/helloamitkr/cvfit-backend/service"
	apperrors "github.com/helloamitkr/cvfit-tools/errors"
)

func (d *Deps) HandleRegister(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, apperrors.BadRequest("invalid JSON body"))
		return
	}
	profile, token, appErr := d.Svc.Register(r.Context(), body.Email, body.Password, body.Name)
	if appErr != nil {
		writeError(w, appErr)
		return
	}
	writeSuccess(w, http.StatusCreated, map[string]any{"user": profile, "token": token})
}

func (d *Deps) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, apperrors.BadRequest("invalid JSON body"))
		return
	}
	profile, token, appErr := d.Svc.Login(r.Context(), body.Email, body.Password)
	if appErr != nil {
		writeError(w, appErr)
		return
	}
	writeSuccess(w, http.StatusOK, map[string]any{"user": profile, "token": token})
}

func (d *Deps) HandleMe(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromCtx(r.Context())
	profile, appErr := d.Svc.Me(r.Context(), userID)
	if appErr != nil {
		writeError(w, appErr)
		return
	}
	writeSuccess(w, http.StatusOK, profile)
}

// HandleRequestOTP sends a 6-digit OTP to the given email address.
// Works for both new and existing accounts — no password required.
func (d *Deps) HandleRequestOTP(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, apperrors.BadRequest("invalid JSON body"))
		return
	}
	if appErr := d.Svc.RequestOTP(r.Context(), body.Email, body.Name, body.Password); appErr != nil {
		writeError(w, appErr)
		return
	}
	writeSuccess(w, http.StatusOK, map[string]any{"message": "OTP sent — check your email"})
}

// HandleVerifyOTP verifies the OTP and returns a JWT on success.
func (d *Deps) HandleVerifyOTP(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, apperrors.BadRequest("invalid JSON body"))
		return
	}
	profile, token, appErr := d.Svc.VerifyOTP(r.Context(), body.Email, body.Code)
	if appErr != nil {
		writeError(w, appErr)
		return
	}
	writeSuccess(w, http.StatusOK, map[string]any{"user": profile, "token": token})
}

func (d *Deps) HandleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromCtx(r.Context())
	var input service.UpdateProfileInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, apperrors.BadRequest("invalid JSON body"))
		return
	}
	u, err := d.Svc.UpdateProfile(r.Context(), userID, input)
	if err != nil || u == nil {
		writeError(w, apperrors.InternalServerWrap("failed to update profile", err))
		return
	}
	writeSuccess(w, http.StatusOK, u)
}
