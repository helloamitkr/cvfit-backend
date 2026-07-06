package model

import "time"

// ResumeRequest is created when a user pays ₹500 for a human-reviewed resume.
// Status: pending → paid → in_progress → completed
type ResumeRequest struct {
	ID          string     `json:"id" dynamodbav:"id"`
	UserID      string     `json:"user_id" dynamodbav:"user_id"`
	Name        string     `json:"name" dynamodbav:"name"`
	Email       string     `json:"email" dynamodbav:"email"`
	ResumeLink  string     `json:"resume_link,omitempty" dynamodbav:"resume_link,omitempty"` // Google Drive / Dropbox / any link
	WantsCall   bool       `json:"wants_call" dynamodbav:"wants_call"`
	Notes       string     `json:"notes,omitempty" dynamodbav:"notes,omitempty"`
	Amount      int        `json:"amount" dynamodbav:"amount"` // paise
	Status      string     `json:"status" dynamodbav:"status"` // pending / paid / in_progress / completed
	PaymentID   string     `json:"payment_id,omitempty" dynamodbav:"payment_id,omitempty"`
	PaidAt      *time.Time `json:"paid_at,omitempty" dynamodbav:"paid_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at" dynamodbav:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" dynamodbav:"updated_at"`
}
