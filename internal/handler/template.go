package handler

import (
	"context"

	cvfitv1 "github.com/helloamitkr/cvfit-tools/gen/cvfit/v1"
	"github.com/twitchtv/twirp"
)

var builtinTemplates = []*cvfitv1.TemplateInfo{
	{
		Name:        "modern",
		Label:       "Modern",
		Description: "Clean, accent-colored header with skill tags",
	},
	{
		Name:        "classic",
		Label:       "Classic",
		Description: "Traditional serif-based, black & white",
	},
	{
		Name:        "minimal",
		Label:       "Minimal",
		Description: "Ultra-light, whitespace-heavy",
	},
	{
		Name:        "professional",
		Label:       "Professional",
		Description: "Colored header banner, two-column skills",
	},
}

type TemplateHandler struct{}

func NewTemplateHandler() *TemplateHandler { return &TemplateHandler{} }

func (h *TemplateHandler) ListTemplates(_ context.Context, _ *cvfitv1.ListTemplatesRequest) (*cvfitv1.ListTemplatesResponse, error) {
	return &cvfitv1.ListTemplatesResponse{Templates: builtinTemplates}, nil
}

func (h *TemplateHandler) GetTemplate(_ context.Context, req *cvfitv1.GetTemplateRequest) (*cvfitv1.TemplateInfo, error) {
	for _, t := range builtinTemplates {
		if t.Name == req.Name {
			return t, nil
		}
	}
	return nil, twirp.NotFoundError("template not found: " + req.Name)
}
