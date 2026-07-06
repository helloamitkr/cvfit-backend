package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	cvai "github.com/helloamitkr/cvfit-tools/ai"
	apperrors "github.com/helloamitkr/cvfit-tools/errors"
)

// checkAIQuota enforces the per-user daily free AI limit for signed-in users.
// Anonymous users are governed by the rate limiter only. Returns true if the
// request may proceed; otherwise it writes a 402 (Payment Required) and returns false.
func (d *Deps) checkAIQuota(w http.ResponseWriter, r *http.Request) bool {
	userID := UserIDFromCtx(r.Context())
	if userID == "" {
		return true
	}
	allowed, _, limit := d.Svc.CheckAIQuota(r.Context(), userID)
	if !allowed {
		writeError(w, apperrors.New(http.StatusPaymentRequired,
			fmt.Sprintf("You've used your %d free AI generations today. Unlock unlimited AI and a watermark-free PDF for ₹21.", limit)))
		return false
	}
	return true
}

// recordAIUsage counts a successful generation against a signed-in user's daily allowance.
func (d *Deps) recordAIUsage(r *http.Request) {
	if userID := UserIDFromCtx(r.Context()); userID != "" {
		d.Svc.ConsumeAIQuota(r.Context(), userID)
	}
}

func (d *Deps) HandleAIEnhanceProject(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, apperrors.BadRequest("invalid JSON body"))
		return
	}
	if !d.checkAIQuota(w, r) {
		return
	}
	result, appErr := cvai.EnhanceProject(r.Context(), body.Description)
	if appErr != nil {
		writeError(w, appErr)
		return
	}
	d.recordAIUsage(r)
	writeSuccess(w, http.StatusOK, result)
}

func (d *Deps) HandleAIExperienceHighlights(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Position    string `json:"position"`
		Company     string `json:"company"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, apperrors.BadRequest("invalid JSON body"))
		return
	}
	if !d.checkAIQuota(w, r) {
		return
	}
	result, appErr := cvai.GenerateExperienceHighlights(r.Context(), body.Position, body.Company, body.Description)
	if appErr != nil {
		writeError(w, appErr)
		return
	}
	d.recordAIUsage(r)
	writeSuccess(w, http.StatusOK, result)
}

func (d *Deps) HandleAISkillsFromRole(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, apperrors.BadRequest("invalid JSON body"))
		return
	}
	if !d.checkAIQuota(w, r) {
		return
	}
	skills, appErr := cvai.GenerateSkillsFromRole(r.Context(), body.Role)
	if appErr != nil {
		writeError(w, appErr)
		return
	}
	d.recordAIUsage(r)
	writeSuccess(w, http.StatusOK, map[string]interface{}{"skills": skills})
}

func (d *Deps) HandleAIGenerate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		JobDescription string `json:"job_description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, apperrors.BadRequest("invalid JSON body"))
		return
	}
	if !d.checkAIQuota(w, r) {
		return
	}
	result, appErr := cvai.GenerateResumeCore(r.Context(), body.JobDescription)
	if appErr != nil {
		writeError(w, appErr)
		return
	}
	d.recordAIUsage(r)
	writeSuccess(w, http.StatusOK, result)
}

func (d *Deps) HandleAICoverLetter(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ResumeText     string `json:"resume_text"`
		JobDescription string `json:"job_description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, apperrors.BadRequest("invalid JSON body"))
		return
	}
	result, appErr := cvai.GenerateCoverLetter(r.Context(), body.ResumeText, body.JobDescription)
	if appErr != nil {
		writeError(w, appErr)
		return
	}
	writeSuccess(w, http.StatusOK, result)
}

func (d *Deps) HandleAILinkedIn(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ResumeText string `json:"resume_text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, apperrors.BadRequest("invalid JSON body"))
		return
	}
	result, appErr := cvai.OptimizeLinkedIn(r.Context(), body.ResumeText)
	if appErr != nil {
		writeError(w, appErr)
		return
	}
	writeSuccess(w, http.StatusOK, result)
}

func (d *Deps) HandleAIInterview(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ResumeText     string `json:"resume_text"`
		JobDescription string `json:"job_description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, apperrors.BadRequest("invalid JSON body"))
		return
	}
	result, appErr := cvai.GenerateInterviewQuestions(r.Context(), body.ResumeText, body.JobDescription)
	if appErr != nil {
		writeError(w, appErr)
		return
	}
	writeSuccess(w, http.StatusOK, result)
}

func (d *Deps) HandleAIKeywordGap(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ResumeText     string `json:"resume_text"`
		JobDescription string `json:"job_description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, apperrors.BadRequest("invalid JSON body"))
		return
	}
	result, appErr := cvai.AnalyzeKeywordGap(r.Context(), body.ResumeText, body.JobDescription)
	if appErr != nil {
		writeError(w, appErr)
		return
	}
	writeSuccess(w, http.StatusOK, result)
}

func (d *Deps) HandleAICareerCoach(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ResumeText string `json:"resume_text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, apperrors.BadRequest("invalid JSON body"))
		return
	}
	result, appErr := cvai.GenerateCareerAdvice(r.Context(), body.ResumeText)
	if appErr != nil {
		writeError(w, appErr)
		return
	}
	writeSuccess(w, http.StatusOK, result)
}

func (d *Deps) HandleAIProjectGenerate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TechStack string `json:"tech_stack"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, apperrors.BadRequest("invalid JSON body"))
		return
	}
	result, appErr := cvai.GenerateProject(r.Context(), body.TechStack)
	if appErr != nil {
		writeError(w, appErr)
		return
	}
	writeSuccess(w, http.StatusOK, result)
}

func (d *Deps) HandleAIResumeReview(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ResumeText string `json:"resume_text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, apperrors.BadRequest("invalid JSON body"))
		return
	}
	result, appErr := cvai.ReviewResume(r.Context(), body.ResumeText)
	if appErr != nil {
		writeError(w, appErr)
		return
	}
	writeSuccess(w, http.StatusOK, result)
}
