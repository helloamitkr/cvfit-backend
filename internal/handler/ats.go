package handler

import (
	"context"
	"strings"

	cvfitv1 "github.com/helloamitkr/cvfit-tools/gen/cvfit/v1"
	"github.com/twitchtv/twirp"
)

type ATSHandler struct{}

func NewATSHandler() *ATSHandler { return &ATSHandler{} }

func (h *ATSHandler) ScoreResume(_ context.Context, req *cvfitv1.ScoreResumeRequest) (*cvfitv1.ScoreResumeResponse, error) {
	if len(req.ResumeFile) == 0 {
		return nil, twirp.InvalidArgumentError("resume_file", "required")
	}

	content := strings.ToLower(string(req.ResumeFile))

	score := scoreContent(content, req.JobDescription)

	return score, nil
}

// scoreContent performs a basic keyword and structure check on the resume text.
// Replace with a real ML/NLP-based scorer in production.
func scoreContent(content string, jobDesc *string) *cvfitv1.ScoreResumeResponse {
	atsSections := []string{"experience", "education", "skills", "summary", "projects", "certifications"}
	commonKeywords := []string{
		"leadership", "communication", "teamwork", "problem-solving",
		"project management", "agile", "scrum", "results", "delivered",
	}

	var matched, missing []string
	sectionCount := 0

	for _, section := range atsSections {
		if strings.Contains(content, section) {
			sectionCount++
		}
	}

	for _, kw := range commonKeywords {
		if strings.Contains(content, kw) {
			matched = append(matched, kw)
		} else {
			missing = append(missing, kw)
		}
	}

	if jobDesc != nil && *jobDesc != "" {
		jd := strings.ToLower(*jobDesc)
		words := strings.Fields(jd)
		seen := map[string]bool{}
		for _, w := range words {
			w = strings.Trim(w, ".,;:")
			if len(w) > 4 && !seen[w] {
				seen[w] = true
				if strings.Contains(content, w) {
					matched = append(matched, w)
				} else {
					missing = append(missing, w)
				}
			}
		}
	}

	keywordScore := int32(0)
	if len(matched)+len(missing) > 0 {
		keywordScore = int32(float64(len(matched)) / float64(len(matched)+len(missing)) * 100)
	}

	formattingScore := int32(60)
	if strings.Contains(content, "@") {
		formattingScore += 10
	}
	if strings.Contains(content, "http") {
		formattingScore += 10
	}
	if formattingScore > 100 {
		formattingScore = 100
	}

	sectionScore := int32(float64(sectionCount) / float64(len(atsSections)) * 100)

	overall := (keywordScore + formattingScore + sectionScore) / 3

	var suggestions []string
	if keywordScore < 50 {
		suggestions = append(suggestions, "Add more industry-relevant keywords to improve ATS matching.")
	}
	if sectionScore < 80 {
		suggestions = append(suggestions, "Ensure your resume has all standard sections: Experience, Education, Skills, and Summary.")
	}
	if !strings.Contains(content, "@") {
		suggestions = append(suggestions, "Add a contact email address.")
	}
	if len(matched) == 0 && jobDesc != nil {
		suggestions = append(suggestions, "Tailor your resume to include keywords from the job description.")
	}

	return &cvfitv1.ScoreResumeResponse{
		OverallScore:        overall,
		KeywordMatch:        keywordScore,
		FormattingScore:     formattingScore,
		SectionCompleteness: sectionScore,
		Suggestions:         suggestions,
		MatchedKeywords:     matched,
		MissingKeywords:     missing,
	}
}
