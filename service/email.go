package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/textproto"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sestypes "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

// EmailSender wraps AWS SES v2 for transactional emails.
type EmailSender struct {
	client    *sesv2.Client
	fromEmail string // verified SES identity, e.g. "CVFit <noreply@yourdomain.com>"
	fromName  string
}

// NewEmailSender creates an SES v2 client using ambient AWS credentials.
func NewEmailSender(ctx context.Context, region, fromEmail, fromName string) (*EmailSender, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("ses: load aws config: %w", err)
	}
	return &EmailSender{
		client:    sesv2.NewFromConfig(cfg),
		fromEmail: fromEmail,
		fromName:  fromName,
	}, nil
}

func (e *EmailSender) fromHeader() string {
	if e.fromName == "" {
		return e.fromEmail
	}
	enc := mime.BEncoding.Encode("utf-8", e.fromName)
	return enc + " <" + e.fromEmail + ">"
}

// SendOTP sends a 6-digit one-time password email.
func (e *EmailSender) SendOTP(ctx context.Context, toEmail, toName, code string) error {
	greeting := "Hello"
	if toName != "" {
		greeting = "Hello " + toName
	}

	htmlBody := `<!DOCTYPE html>
<html>
<body style="font-family:Arial,sans-serif;background:#f9fafb;margin:0;padding:32px">
  <div style="max-width:440px;margin:0 auto;background:#fff;border-radius:12px;padding:32px;box-shadow:0 2px 12px rgba(0,0,0,0.07)">
    <h2 style="color:#1e293b;margin:0 0 8px">Your ZustResume login code</h2>
    <p style="color:#64748b;margin:0 0 24px">` + greeting + `,<br>Use the code below to sign in. It expires in <strong>10 minutes</strong>.</p>
    <div style="background:#eff6ff;border:1px solid #bfdbfe;border-radius:10px;padding:20px 28px;text-align:center;margin-bottom:24px">
      <span style="font-size:38px;font-weight:800;letter-spacing:14px;color:#1d4ed8">` + code + `</span>
    </div>
    <p style="color:#94a3b8;font-size:13px;margin:0">If you didn't request this, you can safely ignore this email.</p>
  </div>
</body>
</html>`

	textBody := greeting + ",\n\nYour ZustResume login code is: " + code + "\n\nThis code expires in 10 minutes.\n\nIf you didn't request this, ignore this email."

	return e.sendSimple(ctx, toEmail, "Your ZustResume login code: "+code, textBody, htmlBody)
}

// SendWelcome sends a welcome email after account creation.
func (e *EmailSender) SendWelcome(ctx context.Context, toEmail, toName string) error {
	greeting := "Hi there"
	if toName != "" {
		greeting = "Hi " + toName
	}

	htmlBody := `<!DOCTYPE html>
<html>
<body style="font-family:Arial,sans-serif;background:#f9fafb;margin:0;padding:32px">
  <div style="max-width:440px;margin:0 auto;background:#fff;border-radius:12px;padding:32px;box-shadow:0 2px 12px rgba(0,0,0,0.07)">
    <h2 style="color:#1e293b;margin:0 0 8px">Welcome to ZustResume!</h2>
    <p style="color:#64748b;margin:0 0 16px">` + greeting + `, your account is ready.</p>
    <p style="color:#64748b;margin:0 0 24px">Build ATS-optimised resumes in minutes — pick a template, let AI fill your summary and skills, and download a pixel-perfect PDF.</p>
    <a href="https://zustresume.com/builder" style="display:inline-block;background:#2563eb;color:#fff;text-decoration:none;border-radius:8px;padding:12px 24px;font-weight:600;font-size:15px">Build my resume →</a>
    <p style="color:#94a3b8;font-size:13px;margin-top:24px">Questions? Reply to this email — we read every one.</p>
  </div>
</body>
</html>`

	textBody := greeting + ",\n\nWelcome to ZustResume! Build ATS-optimised resumes at https://zustresume.com/builder"

	return e.sendSimple(ctx, toEmail, "Welcome to ZustResume!", textBody, htmlBody)
}

// SendResumePDF emails the generated PDF as an attachment.
func (e *EmailSender) SendResumePDF(ctx context.Context, toEmail, toName string, pdfBytes []byte, filename string) error {
	greeting := "Hi there"
	if toName != "" {
		greeting = "Hi " + toName
	}

	htmlPart := `<!DOCTYPE html>
<html>
<body style="font-family:Arial,sans-serif;background:#f9fafb;margin:0;padding:32px">
  <div style="max-width:440px;margin:0 auto;background:#fff;border-radius:12px;padding:32px;box-shadow:0 2px 12px rgba(0,0,0,0.07)">
    <h2 style="color:#1e293b;margin:0 0 8px">Your resume is ready!</h2>
    <p style="color:#64748b;margin:0 0 16px">` + greeting + `,</p>
    <p style="color:#64748b;margin:0 0 24px">Your resume PDF is attached to this email. Good luck with your application!</p>
    <p style="color:#94a3b8;font-size:13px">Need to make changes? Head back to <a href="https://zustresume.com/builder" style="color:#2563eb">zustresume.com/builder</a> anytime.</p>
  </div>
</body>
</html>`

	textPart := greeting + ",\n\nYour resume PDF is attached. Good luck!\n\nNeed changes? Visit https://zustresume.com/builder"

	raw, err := buildRawEmailWithAttachment(
		e.fromHeader(), toEmail,
		"Your ZustResume PDF is ready",
		textPart, htmlPart,
		filename, pdfBytes,
	)
	if err != nil {
		return fmt.Errorf("ses: build mime: %w", err)
	}

	_, err = e.client.SendEmail(ctx, &sesv2.SendEmailInput{
		Content: &sestypes.EmailContent{
			Raw: &sestypes.RawMessage{Data: raw},
		},
	})
	return err
}

// sendSimple sends a plain text + HTML email without attachments.
func (e *EmailSender) sendSimple(ctx context.Context, toEmail, subject, textBody, htmlBody string) error {
	_, err := e.client.SendEmail(ctx, &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(e.fromHeader()),
		Destination: &sestypes.Destination{
			ToAddresses: []string{toEmail},
		},
		Content: &sestypes.EmailContent{
			Simple: &sestypes.Message{
				Subject: &sestypes.Content{
					Data:    aws.String(subject),
					Charset: aws.String("UTF-8"),
				},
				Body: &sestypes.Body{
					Text: &sestypes.Content{
						Data:    aws.String(textBody),
						Charset: aws.String("UTF-8"),
					},
					Html: &sestypes.Content{
						Data:    aws.String(htmlBody),
						Charset: aws.String("UTF-8"),
					},
				},
			},
		},
	})
	return err
}

// EmailResumePDF looks up the authenticated user and emails their resume PDF.
// Designed to be called in a goroutine — all errors are logged, not returned.
func (s *Service) EmailResumePDF(ctx context.Context, userID, filename string, pdfBytes []byte) {
	if s.Email == nil {
		return
	}
	u, err := s.GetByID(ctx, userID)
	if err != nil || u == nil {
		return
	}
	name := u.Name
	if u.FirstName != "" {
		name = u.FirstName
	}
	_ = s.Email.SendResumePDF(ctx, u.Email, name, pdfBytes, filename)
}

// buildRawEmailWithAttachment constructs a MIME multipart email with a PDF attachment.
func buildRawEmailWithAttachment(from, to, subject, textBody, htmlBody, attachFilename string, attachData []byte) ([]byte, error) {
	var buf bytes.Buffer

	// Top-level mixed boundary
	mixedWriter := multipart.NewWriter(&buf)
	mixedBoundary := mixedWriter.Boundary()

	// Headers
	buf.Reset()
	hdr := &bytes.Buffer{}
	fmt.Fprintf(hdr, "From: %s\r\n", from)
	fmt.Fprintf(hdr, "To: %s\r\n", to)
	fmt.Fprintf(hdr, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", subject))
	fmt.Fprintf(hdr, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(hdr, "Content-Type: multipart/mixed; boundary=%q\r\n", mixedBoundary)
	fmt.Fprintf(hdr, "\r\n")

	body := &bytes.Buffer{}
	mixedWriter = multipart.NewWriter(body)
	_ = mixedWriter.SetBoundary(mixedBoundary)

	// Part 1: multipart/alternative (text + html)
	altBoundary := "alt_" + strings.ReplaceAll(mixedBoundary, "-", "")
	altHeader := textproto.MIMEHeader{}
	altHeader.Set("Content-Type", "multipart/alternative; boundary="+altBoundary)
	altPart, err := mixedWriter.CreatePart(altHeader)
	if err != nil {
		return nil, err
	}

	altWriter := multipart.NewWriter(altPart)
	_ = altWriter.SetBoundary(altBoundary)

	// text/plain
	plainHeader := textproto.MIMEHeader{}
	plainHeader.Set("Content-Type", "text/plain; charset=utf-8")
	plainHeader.Set("Content-Transfer-Encoding", "quoted-printable")
	plainPart, err := altWriter.CreatePart(plainHeader)
	if err != nil {
		return nil, err
	}
	qpw := quotedprintable.NewWriter(plainPart)
	_, _ = qpw.Write([]byte(textBody))
	_ = qpw.Close()

	// text/html
	htmlHeader := textproto.MIMEHeader{}
	htmlHeader.Set("Content-Type", "text/html; charset=utf-8")
	htmlHeader.Set("Content-Transfer-Encoding", "quoted-printable")
	htmlPart, err := altWriter.CreatePart(htmlHeader)
	if err != nil {
		return nil, err
	}
	qpw = quotedprintable.NewWriter(htmlPart)
	_, _ = qpw.Write([]byte(htmlBody))
	_ = qpw.Close()

	_ = altWriter.Close()

	// Part 2: PDF attachment
	attachHeader := textproto.MIMEHeader{}
	attachHeader.Set("Content-Type", "application/pdf")
	attachHeader.Set("Content-Transfer-Encoding", "base64")
	attachHeader.Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, attachFilename))
	attachPart, err := mixedWriter.CreatePart(attachHeader)
	if err != nil {
		return nil, err
	}
	enc := base64.NewEncoder(base64.StdEncoding, attachPart)
	_, _ = enc.Write(attachData)
	_ = enc.Close()

	_ = mixedWriter.Close()

	return append(hdr.Bytes(), body.Bytes()...), nil
}
