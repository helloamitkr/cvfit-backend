package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"

	"github.com/helloamitkr/cvfit-backend/handlers"
	"github.com/helloamitkr/cvfit-backend/service"
	"github.com/helloamitkr/cvfit-tools/awsutil"
	"github.com/helloamitkr/cvfit-tools/resume"
)

func main() {
	ctx := context.Background()

	awsCfg, err := awsutil.LoadAWSConfig(ctx, os.Getenv("AWS_REGION"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load AWS config: %v\n", err)
		os.Exit(1)
	}

	// In Lambda, DynamoDB is reached via the AWS SDK directly (no local endpoint).
	dynamo := awsutil.NewDynamoDBClientWithEndpoint(awsCfg, "")

	reg := resume.NewTemplateRegistry()
	if err := resume.RegisterBuiltinTemplates(reg); err != nil {
		fmt.Fprintf(os.Stderr, "failed to register templates: %v\n", err)
		os.Exit(1)
	}

	// Email sender — gracefully disabled if SES_FROM_EMAIL is not set.
	var emailSender *service.EmailSender
	if fromEmail := env("SES_FROM_EMAIL", ""); fromEmail != "" {
		sesRegion := env("AWS_SES_REGION", env("AWS_REGION", "ap-south-1"))
		emailSender, _ = service.NewEmailSender(ctx, sesRegion, fromEmail, "ZustResume")
	}

	svc := &service.Service{
		Dynamo:            dynamo,
		Templates:         reg,
		UsersTableName:    env("USERS_TABLE_NAME", "users"),
		OrdersTableName:   env("ORDERS_TABLE_NAME", "orders"),
		RequestsTableName: env("REQUESTS_TABLE_NAME", "requests"),
		JWTSecret:         env("JWT_SECRET", ""),
		FrontendURL:       env("FRONTEND_URL", ""),
		RazorpayKeyID:     env("RAZORPAY_KEY_ID", ""),
		RazorpayKeySecret: env("RAZORPAY_KEY_SECRET", ""),
		PaymentAmountINR:  2100, // ₹21 in paise
		AdminEmails:       splitCSV(env("ADMIN_EMAILS", "")),
		OAuthProviders:    buildOAuthProviders(),
		Email:             emailSender,
	}

	d := &handlers.Deps{Svc: svc}
	mux := http.NewServeMux()
	registerRoutes(mux, d)

	adapter := httpadapter.NewV2(handlers.CORS(mux))
	lambda.Start(adapter.ProxyWithContext)
}

func registerRoutes(mux *http.ServeMux, d *handlers.Deps) {
	// Rate limiter shared by all AI-backed endpoints (caps cost / abuse).
	// Tool endpoints are keyed per-IP; builder endpoints (wrapped in OptionalAuth)
	// are keyed per-user via aiLimit so signed-in users get their own allowance.
	ai := handlers.NewRateLimiter(aiRateLimitPerMin(), time.Minute)
	aiLimit := ai.MiddlewareBy(handlers.AIRateKey)

	mux.HandleFunc("POST /api/auth/register", d.HandleRegister)
	mux.HandleFunc("POST /api/auth/login", d.HandleLogin)
	mux.HandleFunc("GET /api/auth/me", d.RequireAuth(d.HandleMe))

	mux.HandleFunc("POST /api/auth/otp/request", d.HandleRequestOTP)
	mux.HandleFunc("POST /api/auth/otp/verify", d.HandleVerifyOTP)

	mux.HandleFunc("GET /api/auth/oauth/{provider}", d.HandleOAuthBegin)
	mux.HandleFunc("GET /api/auth/oauth/{provider}/callback", d.HandleOAuthCallback)

	mux.HandleFunc("GET /api/templates", d.HandleListTemplates)

	mux.HandleFunc("POST /api/resumes/render", d.OptionalAuth(d.HandleRenderResume))
	mux.HandleFunc("POST /api/resumes/pdf", d.OptionalAuth(d.HandleGeneratePDF))
	mux.HandleFunc("POST /api/resumes/import", d.RequireAuth(d.HandleImportResume)) // rule-based, no AI

	mux.HandleFunc("POST /api/ats/score", d.HandleATSScore)
	mux.HandleFunc("POST /api/ats/score-resume", d.HandleATSScoreResume)

	// AI generation — rate limited per IP (shared limiter declared above).
	// Builder AI endpoints enforce the per-user daily free limit (OptionalAuth → userID).
	mux.HandleFunc("POST /api/ai/generate", d.OptionalAuth(aiLimit(d.HandleAIGenerate)))
	mux.HandleFunc("POST /api/ai/skills", d.OptionalAuth(aiLimit(d.HandleAISkillsFromRole)))
	mux.HandleFunc("POST /api/ai/project", d.OptionalAuth(aiLimit(d.HandleAIEnhanceProject)))
	mux.HandleFunc("POST /api/ai/experience-highlights", d.OptionalAuth(aiLimit(d.HandleAIExperienceHighlights)))
	mux.HandleFunc("POST /api/ai/cover-letter", ai.Middleware(d.HandleAICoverLetter))
	mux.HandleFunc("POST /api/ai/linkedin", ai.Middleware(d.HandleAILinkedIn))
	mux.HandleFunc("POST /api/ai/interview", ai.Middleware(d.HandleAIInterview))
	mux.HandleFunc("POST /api/ai/keyword-gap", ai.Middleware(d.HandleAIKeywordGap))
	mux.HandleFunc("POST /api/ai/career-coach", ai.Middleware(d.HandleAICareerCoach))
	mux.HandleFunc("POST /api/ai/project-generate", ai.Middleware(d.HandleAIProjectGenerate))
	mux.HandleFunc("POST /api/ai/resume-review", ai.Middleware(d.HandleAIResumeReview))

	mux.HandleFunc("POST /api/payment/create-order", d.RequireAuth(d.HandleCreateOrder))
	mux.HandleFunc("POST /api/payment/verify", d.RequireAuth(d.HandleVerifyPayment))
	mux.HandleFunc("GET /api/payment/status", d.OptionalAuth(d.HandlePaymentStatus))
	mux.HandleFunc("POST /api/payment/cancel", d.OptionalAuth(d.HandleOrderFailed))

	mux.HandleFunc("PATCH /api/users/me", d.RequireAuth(d.HandleUpdateProfile))

	mux.HandleFunc("POST /api/requests", d.RequireAuth(d.HandleCreateResumeRequest))
	mux.HandleFunc("POST /api/requests/verify", d.RequireAuth(d.HandleVerifyRequestPayment))

	mux.HandleFunc("GET /api/docs", handlers.HandleSwaggerUI)
	mux.HandleFunc("GET /api/docs/openapi.yaml", handlers.HandleOpenAPISpec)

	mux.HandleFunc("GET /api/admin/me", d.RequireAuth(d.HandleAdminMe))
	mux.HandleFunc("GET /api/admin/stats", d.RequireAdmin(d.HandleAdminStats))
	mux.HandleFunc("GET /api/admin/users", d.RequireAdmin(d.HandleAdminUsers))
	mux.HandleFunc("GET /api/admin/orders", d.RequireAdmin(d.HandleAdminOrders))
	mux.HandleFunc("GET /api/admin/requests", d.RequireAdmin(d.HandleAdminRequests))
}

func buildOAuthProviders() map[string]service.OAuthProviderConfig {
	providers := map[string]service.OAuthProviderConfig{}
	redirectBase := env("OAUTH_REDIRECT_BASE", "")

	if id := os.Getenv("GOOGLE_CLIENT_ID"); id != "" {
		providers["google"] = service.OAuthProviderConfig{
			ClientID:     id,
			ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
			RedirectBase: redirectBase,
		}
	}
	if id := os.Getenv("GITHUB_CLIENT_ID"); id != "" {
		providers["github"] = service.OAuthProviderConfig{
			ClientID:     id,
			ClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
			RedirectBase: redirectBase,
		}
	}
	if id := os.Getenv("LINKEDIN_CLIENT_ID"); id != "" {
		providers["linkedin"] = service.OAuthProviderConfig{
			ClientID:     id,
			ClientSecret: os.Getenv("LINKEDIN_CLIENT_SECRET"),
			RedirectBase: redirectBase,
		}
	}
	return providers
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// splitCSV parses a comma-separated env value into a trimmed, non-empty slice.
func splitCSV(v string) []string {
	var out []string
	for _, p := range strings.Split(v, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// aiRateLimitPerMin is the per-IP request cap for AI endpoints, overridable via env.
// Default of 15/min matches Gemini Flash's free-tier request rate.
func aiRateLimitPerMin() int {
	if v := os.Getenv("AI_RATE_LIMIT_PER_MIN"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 15
}
