package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	apperrors "github.com/helloamitkr/cvfit-tools/errors"
)

type apiResponse struct {
	Success bool      `json:"success"`
	Data    any       `json:"data,omitempty"`
	Error   *apiError `json:"error,omitempty"`
}

type apiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func writeSuccess(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(apiResponse{Success: true, Data: data})
}

func writeError(w http.ResponseWriter, err *apperrors.AppError) {
	// Log server-side failures (5xx) with the underlying cause so issues like
	// Gemini errors are visible in server logs instead of being swallowed.
	if err.Code >= http.StatusInternalServerError {
		log.Printf("[error] %s", err.Error())
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.Code)
	_ = json.NewEncoder(w).Encode(apiResponse{
		Success: false,
		Error:   &apiError{Code: err.Code, Message: err.Message},
	})
}
