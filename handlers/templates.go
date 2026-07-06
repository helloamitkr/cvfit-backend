package handlers

import "net/http"

// TemplateInfo describes a resume template shown in the UI.
type TemplateInfo struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

func (d *Deps) HandleListTemplates(w http.ResponseWriter, r *http.Request) {
	writeSuccess(w, http.StatusOK, []TemplateInfo{
		{Name: "modern",    Label: "Modern",    Description: "Dark header · accent skill tags · bordered project cards"},
		{Name: "universal", Label: "Universal", Description: "Centered header · accent underline titles · left-border projects"},
		{Name: "standard",  Label: "Standard",  Description: "Skills by category · accent top-border projects · section rules"},
		{Name: "blueprint", Label: "Blueprint", Description: "Classic professional layout with section dividers"},
	})
}
