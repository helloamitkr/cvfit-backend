package service

import (
	"context"
	"os"

	cvai "github.com/helloamitkr/cvfit-tools/ai"
	cvats "github.com/helloamitkr/cvfit-tools/ats"
	apperrors "github.com/helloamitkr/cvfit-tools/errors"
	"github.com/helloamitkr/cvfit-tools/resume"
)

func ScoreATS(ctx context.Context, fileBytes []byte, _ string, jobDescription string) (*cvats.ATSResult, *apperrors.AppError) {
	var scorer cvats.AIScorer
	if os.Getenv("OLLAMA_ENDPOINT") != "" || os.Getenv("GEMINI_API_KEY") != "" {
		scorer = aiScorer{}
	}
	return cvats.ScoreFromBytes(ctx, fileBytes, jobDescription, scorer)
}

func ScoreATSFromResume(rv resume.Resume, jobDescription string) *cvats.ATSResult {
	return cvats.ScoreFromResume(rv, jobDescription)
}

type aiScorer struct{}

func (aiScorer) Score(ctx context.Context, resumeText, jobDescription string) (*cvats.ATSResult, error) {
	return cvai.ScoreWithAI(ctx, resumeText, jobDescription)
}
