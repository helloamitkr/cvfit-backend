# cvfit-backend

Go REST API for ZustResume (CVFit). Runs two ways from one codebase:

- **`cmd/server`** — local `net/http` server (`http://localhost:8080`)
- **`cmd/lambda`** — AWS Lambda behind API Gateway (container image)

Business logic lives in `service/`, HTTP handlers in `handlers/`; shared domain code (resume templates, AI, ATS, PDF, parsing) comes from [`cvfit-tools`](../cvfit-tools) via a `replace` directive.

## Run locally

```bash
cp .env.example .env.local          # set the vars below
go run ./cmd/server                 # :8080
```

Requires local **DynamoDB** on `:8000` (dev auto-creates tables) and **Chrome** for PDF rendering.

### Key environment variables

| Var | Purpose |
|-----|---------|
| `GEMINI_API_KEY` | Enables Gemini AI (else falls back to Ollama at `OLLAMA_ENDPOINT`) |
| `JWT_SECRET` | Signs auth tokens |
| `RAZORPAY_KEY_ID` / `RAZORPAY_KEY_SECRET` | Payments |
| `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` | Google OAuth (GitHub/LinkedIn analogous) |
| `SES_FROM_EMAIL` / `AWS_SES_REGION` | Transactional email (optional) |
| `ADMIN_EMAILS` | Comma-separated emails granted admin (no DynamoDB `is_admin` flag needed) |
| `FREE_AI_PER_DAY` | Free AI generations/day per user (default `3`) |
| `AI_RATE_LIMIT_PER_MIN` | Per-IP AI rate limit (default `15`) |
| `DYNAMODB_ENDPOINT` | Local DynamoDB URL (dev) |

## API surface

Responses are wrapped as `{ "success": bool, "data": … }` or `{ "success": false, "error": { code, message } }`.

**Auth** — `POST /api/auth/register` · `/login` · `/otp/request` · `/otp/verify` · `GET /api/auth/me` · `GET /api/auth/oauth/{provider}` (+ `/callback`)

**Resumes** — `POST /api/resumes/render` · `POST /api/resumes/pdf` (watermarked unless a paid `order_id` is passed) · `POST /api/resumes/import` (rule-based parse, no AI) · `GET /api/templates`

**AI** (rate-limited; builder endpoints enforce the daily free limit) — `POST /api/ai/generate` · `/skills` · `/project` · `/cover-letter` · `/linkedin` · `/interview` · `/keyword-gap` · `/career-coach` · `/project-generate` · `/resume-review`

**ATS** — `POST /api/ats/score` (file upload) · `POST /api/ats/score-resume` (structured)

**Payments (Razorpay)** — `POST /api/payment/create-order` · `/verify` · `/cancel` · `GET /api/payment/status`. Verifying a payment also **grants unlimited AI for the day** (the ₹21 bundle).

**Resume requests** ("Ask Us to Create", ₹200) — `POST /api/requests` · `/requests/verify`

**Profile** — `PATCH /api/users/me`

**Admin** (requires `is_admin`) — `GET /api/admin/stats` · `/users` · `/orders` · `/requests` · `/me`

**Docs** — `GET /api/docs` (Swagger UI) · `GET /api/docs/openapi.yaml`

## Notable design points

- **AI routing** — `cvfit-tools/ai` uses Gemini when `GEMINI_API_KEY` is set, else Ollama. Errors (quota, 5xx) are retried and logged; a 5xx surfaces its cause in server logs.
- **Daily AI limit** — `service/aiusage.go` tracks per-user usage on the user record (`ai_usage_date/count`, `ai_paid_date`); fails open on DB errors.
- **Watermark gating** — `service/pdf.go` adds a "PREVIEW" watermark unless `HasRecentPayment(userID, orderID)` confirms a paid order via a strongly-consistent `GetItem`.
- **Rate limiting** — `handlers/ratelimit.go` (per-IP, in-memory) on all `/api/ai/*` routes.

## Build

```bash
go build ./...        # both cmds + handlers + services
go vet ./...
```