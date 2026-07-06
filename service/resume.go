package service

import (
	"context"

	apperrors "github.com/helloamitkr/cvfit-tools/errors"
	"github.com/helloamitkr/cvfit-tools/resume"
)

func (s *Service) RenderResume(ctx context.Context, r *resume.Resume) (string, *apperrors.AppError) {
	return s.Templates.RenderSafe(ctx, r)
}
