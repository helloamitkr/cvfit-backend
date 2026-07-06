package model

import "time"

// Order records every Razorpay order and, on success, the completed payment.
// Status transitions: pending → success | failed
// s3_url and is_email_sent are set after a successful payment receipt is generated.
type Order struct {
	ID          string     `json:"id" dynamodbav:"id"`                                    // Razorpay order_id (PK)
	UserID      string     `json:"user_id" dynamodbav:"user_id"`
	UserEmail   string     `json:"user_email" dynamodbav:"user_email"`
	ResumeID    string     `json:"resume_id" dynamodbav:"resume_id"`
	Amount      int        `json:"amount" dynamodbav:"amount"`                            // paise
	Status      string     `json:"status" dynamodbav:"status"`                            // pending / success / failed
	PaymentID   string     `json:"payment_id,omitempty" dynamodbav:"payment_id,omitempty"` // set on success
	PaidAt      *time.Time `json:"paid_at,omitempty" dynamodbav:"paid_at,omitempty"`       // set on success
	S3URL       string     `json:"s3_url,omitempty" dynamodbav:"s3_url,omitempty"`         // receipt PDF in S3
	IsEmailSent bool       `json:"is_email_sent" dynamodbav:"is_email_sent"`               // receipt email sent
	CreatedAt   time.Time  `json:"created_at" dynamodbav:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" dynamodbav:"updated_at"`
}
