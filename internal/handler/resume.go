package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/google/uuid"
	"github.com/helloamitkr/cvfit-tools/awsutil"
	cvfitv1 "github.com/helloamitkr/cvfit-tools/gen/cvfit/v1"
	"github.com/helloamitkr/cvfit-tools/resume"
	"github.com/twitchtv/twirp"
)

// resumeRecord is the DynamoDB item shape.
// Keys must match the table/GSI definitions in Terraform and localstack/init.sh.
type resumeRecord struct {
	ID        string `dynamodbav:"id"`
	UserID    string `dynamodbav:"userId"`    // GSI hash key
	CreatedAt string `dynamodbav:"createdAt"` // GSI sort key
	UpdatedAt string `dynamodbav:"updatedAt"`
	Data      string `dynamodbav:"data"` // JSON-encoded cvfitv1.Resume
}

type ResumeHandler struct {
	registry  *resume.TemplateRegistry
	db        *awsutil.DynamoDBClient
	tableName string
}

func NewResumeHandler(db *awsutil.DynamoDBClient, tableName string) *ResumeHandler {
	reg := resume.NewTemplateRegistry()
	if err := resume.RegisterBuiltinTemplates(reg); err != nil {
		panic(fmt.Sprintf("register templates: %v", err))
	}
	return &ResumeHandler{registry: reg, db: db, tableName: tableName}
}

func (h *ResumeHandler) hasDynamo() bool {
	return h.db != nil && h.tableName != ""
}

func userIDFrom(r *cvfitv1.Resume) string {
	if r.UserId != nil && *r.UserId != "" {
		return *r.UserId
	}
	return "anonymous"
}

func (h *ResumeHandler) CreateResume(ctx context.Context, req *cvfitv1.CreateResumeRequest) (*cvfitv1.Resume, error) {
	if req.Resume == nil {
		return nil, twirp.InvalidArgumentError("resume", "required")
	}
	r := req.Resume
	r.Id = ptr(uuid.New().String())
	r.Status = ptr("draft")

	if !h.hasDynamo() {
		return r, nil
	}

	now := time.Now().UTC().Format(time.RFC3339)
	data, err := json.Marshal(r)
	if err != nil {
		return nil, twirp.InternalErrorWith(err)
	}
	rec := resumeRecord{
		ID:        *r.Id,
		UserID:    userIDFrom(r),
		CreatedAt: now,
		UpdatedAt: now,
		Data:      string(data),
	}
	if err := h.db.PutItem(ctx, h.tableName, rec); err != nil {
		return nil, twirp.InternalErrorWith(err)
	}
	return r, nil
}

func (h *ResumeHandler) GetResume(ctx context.Context, req *cvfitv1.GetResumeRequest) (*cvfitv1.Resume, error) {
	if req.Id == "" {
		return nil, twirp.InvalidArgumentError("id", "required")
	}
	if !h.hasDynamo() {
		return nil, twirp.NotFoundError("resume not found")
	}

	var rec resumeRecord
	if err := h.db.GetItemByStringKey(ctx, h.tableName, map[string]string{"id": req.Id}, &rec); err != nil {
		return nil, twirp.InternalErrorWith(err)
	}
	if rec.ID == "" {
		return nil, twirp.NotFoundError("resume not found")
	}

	var r cvfitv1.Resume
	if err := json.Unmarshal([]byte(rec.Data), &r); err != nil {
		return nil, twirp.InternalErrorWith(err)
	}
	return &r, nil
}

func (h *ResumeHandler) UpdateResume(ctx context.Context, req *cvfitv1.UpdateResumeRequest) (*cvfitv1.Resume, error) {
	if req.Id == "" {
		return nil, twirp.InvalidArgumentError("id", "required")
	}
	if req.Resume == nil {
		return nil, twirp.InvalidArgumentError("resume", "required")
	}
	req.Resume.Id = ptr(req.Id)

	if !h.hasDynamo() {
		return req.Resume, nil
	}

	// Fetch existing record to preserve createdAt and userId.
	var existing resumeRecord
	if err := h.db.GetItemByStringKey(ctx, h.tableName, map[string]string{"id": req.Id}, &existing); err != nil {
		return nil, twirp.InternalErrorWith(err)
	}
	if existing.ID == "" {
		return nil, twirp.NotFoundError("resume not found")
	}

	data, err := json.Marshal(req.Resume)
	if err != nil {
		return nil, twirp.InternalErrorWith(err)
	}
	rec := resumeRecord{
		ID:        req.Id,
		UserID:    existing.UserID,
		CreatedAt: existing.CreatedAt,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		Data:      string(data),
	}
	if err := h.db.PutItem(ctx, h.tableName, rec); err != nil {
		return nil, twirp.InternalErrorWith(err)
	}
	return req.Resume, nil
}

func (h *ResumeHandler) DeleteResume(ctx context.Context, req *cvfitv1.DeleteResumeRequest) (*cvfitv1.DeleteResumeResponse, error) {
	if req.Id == "" {
		return nil, twirp.InvalidArgumentError("id", "required")
	}
	if !h.hasDynamo() {
		return &cvfitv1.DeleteResumeResponse{}, nil
	}
	if err := h.db.DeleteItemByStringKey(ctx, h.tableName, map[string]string{"id": req.Id}); err != nil {
		return nil, twirp.InternalErrorWith(err)
	}
	return &cvfitv1.DeleteResumeResponse{}, nil
}

func (h *ResumeHandler) ListResumes(ctx context.Context, req *cvfitv1.ListResumesRequest) (*cvfitv1.ListResumesResponse, error) {
	if !h.hasDynamo() {
		return &cvfitv1.ListResumesResponse{Resumes: []*cvfitv1.Resume{}}, nil
	}

	userID := "anonymous"
	if req.UserId != nil && *req.UserId != "" {
		userID = *req.UserId
	}

	var records []resumeRecord
	if err := h.db.QueryIndex(
		ctx,
		h.tableName,
		"userId-createdAt-index",
		"userId = :uid",
		map[string]string{":uid": userID},
		&records,
	); err != nil {
		return nil, twirp.InternalErrorWith(err)
	}

	resumes := make([]*cvfitv1.Resume, 0, len(records))
	for _, rec := range records {
		var r cvfitv1.Resume
		if err := json.Unmarshal([]byte(rec.Data), &r); err != nil {
			continue
		}
		resumes = append(resumes, &r)
	}
	return &cvfitv1.ListResumesResponse{Resumes: resumes}, nil
}

func (h *ResumeHandler) RenderResume(_ context.Context, req *cvfitv1.RenderResumeRequest) (*cvfitv1.RenderResumeResponse, error) {
	if req.Resume == nil {
		return nil, twirp.InvalidArgumentError("resume", "required")
	}

	r, err := protoToResumeModel(req.Resume)
	if err != nil {
		return nil, twirp.InvalidArgumentError("resume", err.Error())
	}

	tmplName := req.Resume.Template
	if tmplName == "" {
		tmplName = "modern"
	}

	html, err := h.registry.Render(tmplName, r)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, twirp.NotFoundError("template not found: " + tmplName)
		}
		return nil, twirp.InternalErrorWith(err)
	}

	return &cvfitv1.RenderResumeResponse{Html: html}, nil
}

func (h *ResumeHandler) ExportPDF(ctx context.Context, req *cvfitv1.ExportPDFRequest) (*cvfitv1.ExportPDFResponse, error) {
	if req.Resume == nil {
		return nil, twirp.InvalidArgumentError("resume", "required")
	}

	r, err := protoToResumeModel(req.Resume)
	if err != nil {
		return nil, twirp.InvalidArgumentError("resume", err.Error())
	}

	tmplName := req.Resume.Template
	if tmplName == "" {
		tmplName = "modern"
	}

	html, err := h.registry.Render(tmplName, r)
	if err != nil {
		return nil, twirp.InternalErrorWith(err)
	}

	pdfBytes, err := renderHTMLToPDF(ctx, html)
	if err != nil {
		return nil, twirp.InternalErrorWith(fmt.Errorf("PDF generation: %w", err))
	}

	firstName, lastName := "", ""
	if req.Resume.Personal != nil {
		firstName = req.Resume.Personal.FirstName
		lastName = req.Resume.Personal.LastName
	}

	return &cvfitv1.ExportPDFResponse{
		PdfData:  pdfBytes,
		Filename: fmt.Sprintf("%s_%s_resume.pdf", firstName, lastName),
	}, nil
}

func renderHTMLToPDF(ctx context.Context, htmlContent string) ([]byte, error) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoSandbox,
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-setuid-sandbox", true),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()

	taskCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	var pdfBuf []byte
	if err := chromedp.Run(taskCtx,
		chromedp.Navigate("about:blank"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			frameTree, err := page.GetFrameTree().Do(ctx)
			if err != nil {
				return err
			}
			return page.SetDocumentContent(frameTree.Frame.ID, htmlContent).Do(ctx)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			pdfBuf, _, err = page.PrintToPDF().
				WithPrintBackground(true).
				WithPaperWidth(8.27).
				WithPaperHeight(11.69).
				WithMarginTop(0).
				WithMarginBottom(0).
				WithMarginLeft(0).
				WithMarginRight(0).
				Do(ctx)
			return err
		}),
	); err != nil {
		return nil, err
	}

	return pdfBuf, nil
}

// protoToResumeModel converts a proto Resume into the cvfit-tools resume model.
func protoToResumeModel(p *cvfitv1.Resume) (*resume.Resume, error) {
	if p == nil {
		return nil, fmt.Errorf("resume is nil")
	}

	r := &resume.Resume{
		Title:    p.Title,
		Template: p.Template,
		Summary:  p.Summary,
		Metadata: resume.DefaultMetadata(),
	}

	if p.Id != nil {
		r.ID = *p.Id
	}
	if p.UserId != nil {
		r.UserID = *p.UserId
	}
	if p.Status != nil {
		r.Status = *p.Status
	}

	if p.Personal != nil {
		r.Personal = resume.Personal{
			FirstName: p.Personal.FirstName,
			LastName:  p.Personal.LastName,
			Email:     p.Personal.Email,
			Phone:     p.Personal.Phone,
		}
		if p.Personal.Location != nil {
			r.Personal.Location = *p.Personal.Location
		}
		if p.Personal.Linkedin != nil {
			r.Personal.LinkedIn = *p.Personal.Linkedin
		}
		if p.Personal.Github != nil {
			r.Personal.GitHub = *p.Personal.Github
		}
		if p.Personal.Website != nil {
			r.Personal.Website = *p.Personal.Website
		}
		if p.Personal.PhotoUrl != nil {
			r.Personal.PhotoURL = *p.Personal.PhotoUrl
		}
	}

	for _, e := range p.Experience {
		exp := resume.Experience{
			Company:   e.Company,
			Position:  e.Position,
			StartDate: e.StartDate,
			Current:   e.Current,
		}
		if e.Location != nil {
			exp.Location = *e.Location
		}
		if e.EndDate != nil {
			exp.EndDate = *e.EndDate
		}
		if e.Description != nil {
			exp.Description = *e.Description
		}
		exp.Highlights = e.Highlights
		r.Experience = append(r.Experience, exp)
	}

	for _, ed := range p.Education {
		edu := resume.Education{
			Institution: ed.Institution,
			Degree:      ed.Degree,
			Field:       ed.Field,
			StartDate:   ed.StartDate,
		}
		if ed.EndDate != nil {
			edu.EndDate = *ed.EndDate
		}
		if ed.Gpa != nil {
			edu.GPA = *ed.Gpa
		}
		if ed.Description != nil {
			edu.Description = *ed.Description
		}
		r.Education = append(r.Education, edu)
	}

	for _, s := range p.Skills {
		sk := resume.Skill{Name: s.Name}
		if s.Category != nil {
			sk.Category = *s.Category
		}
		if s.Proficiency != nil {
			sk.Proficiency = *s.Proficiency
		}
		r.Skills = append(r.Skills, sk)
	}

	for _, proj := range p.Projects {
		pr := resume.Project{
			Name:         proj.Name,
			Description:  proj.Description,
			Technologies: proj.Technologies,
		}
		if proj.Url != nil {
			pr.URL = *proj.Url
		}
		if proj.StartDate != nil {
			pr.StartDate = *proj.StartDate
		}
		if proj.EndDate != nil {
			pr.EndDate = *proj.EndDate
		}
		r.Projects = append(r.Projects, pr)
	}

	for _, c := range p.Certifications {
		cert := resume.Certification{
			Name:      c.Name,
			Issuer:    c.Issuer,
			IssueDate: c.IssueDate,
		}
		if c.ExpiryDate != nil {
			cert.ExpiryDate = *c.ExpiryDate
		}
		if c.CredentialId != nil {
			cert.CredentialID = *c.CredentialId
		}
		if c.Url != nil {
			cert.URL = *c.Url
		}
		r.Certifications = append(r.Certifications, cert)
	}

	for _, l := range p.Languages {
		r.Languages = append(r.Languages, resume.Language{
			Name:        l.Name,
			Proficiency: l.Proficiency,
		})
	}

	if p.Metadata != nil {
		m := &p.Metadata
		if (*m).FontFamily != nil {
			r.Metadata.FontFamily = *(*m).FontFamily
		}
		if (*m).FontSize != nil {
			r.Metadata.FontSize = int(*(*m).FontSize)
		}
		if (*m).PrimaryColor != nil {
			r.Metadata.PrimaryColor = *(*m).PrimaryColor
		}
		if (*m).AccentColor != nil {
			r.Metadata.AccentColor = *(*m).AccentColor
		}
		if (*m).LineSpacing != nil {
			r.Metadata.LineSpacing = *(*m).LineSpacing
		}
		if (*m).MarginTop != nil {
			r.Metadata.MarginTop = *(*m).MarginTop
		}
		if (*m).MarginBottom != nil {
			r.Metadata.MarginBottom = *(*m).MarginBottom
		}
		if (*m).MarginLeft != nil {
			r.Metadata.MarginLeft = *(*m).MarginLeft
		}
		if (*m).MarginRight != nil {
			r.Metadata.MarginRight = *(*m).MarginRight
		}
	}

	return r, nil
}

func ptr[T any](v T) *T { return &v }
