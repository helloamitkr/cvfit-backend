package service

import (
	"github.com/helloamitkr/cvfit-tools/awsutil"
	"github.com/helloamitkr/cvfit-tools/resume"
)

// OAuthProviderConfig holds OAuth2 credentials for a single provider.
type OAuthProviderConfig struct {
	ClientID     string
	ClientSecret string
	RedirectBase string // e.g. "http://localhost:8080"
}

// Service holds all shared dependencies injected into the business logic layer.
type Service struct {
	Dynamo            *awsutil.DynamoDBClient
	Templates         *resume.TemplateRegistry
	UsersTableName    string
	OrdersTableName   string
	RequestsTableName string
	JWTSecret         string
	OAuthProviders    map[string]OAuthProviderConfig
	FrontendURL       string
	RazorpayKeyID     string
	RazorpayKeySecret string
	PaymentAmountINR  int // in paise, e.g. 2100 = ₹21

	// AdminEmails grants admin access via config (ADMIN_EMAILS env) so you don't
	// have to set is_admin in DynamoDB. Case-insensitive.
	AdminEmails []string

	// Email is nil when SES is not configured (e.g. local dev without AWS creds).
	Email *EmailSender
}
