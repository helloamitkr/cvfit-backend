package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	cvats "github.com/helloamitkr/cvfit-tools/ats"
	"github.com/helloamitkr/cvfit-tools/docx"
	apperrors "github.com/helloamitkr/cvfit-tools/errors"
	"github.com/helloamitkr/cvfit-tools/pdftext"
	"github.com/helloamitkr/cvfit-tools/resume"
)

// extractResumeText pulls text from an uploaded resume file. PDFs go through a
// real PDF text library (free, no AI); anything else uses printable-byte
// extraction. If PDF parsing yields almost nothing (e.g. a scanned/image PDF),
// it falls back so the contact-detail regexes still get a chance.
func extractResumeText(data []byte) string {
	if pdftext.IsPDF(data) {
		if txt, err := pdftext.Extract(data); err == nil && len(strings.TrimSpace(txt)) > 30 {
			return txt
		}
	}
	// .docx is a ZIP container; byte-scraping it yields garbage, so extract the
	// real text from word/document.xml.
	if docx.IsDocx(data) {
		if txt, err := docx.Extract(data); err == nil && strings.TrimSpace(txt) != "" {
			return txt
		}
	}
	return cvats.ExtractTextKeepLines(data)
}

// HandleImportResume parses an existing resume (uploaded file or pasted text) into
// structured resume data using rule-based extraction — no AI, no token cost.
// The frontend shows the result for confirmation before saving.
// POST /api/resumes/import — multipart ("resume" file) OR JSON ({ "resume_text": ... }).
func (d *Deps) HandleImportResume(w http.ResponseWriter, r *http.Request) {
	var resumeText string

	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(5 << 20); err != nil {
			writeError(w, apperrors.BadRequest("failed to parse upload: "+err.Error()))
			return
		}
		file, _, err := r.FormFile("resume")
		if err != nil {
			writeError(w, apperrors.BadRequest("'resume' file field is required"))
			return
		}
		defer file.Close()

		fileBytes, err := io.ReadAll(file)
		if err != nil {
			writeError(w, apperrors.InternalServerWrap("failed to read file", err))
			return
		}
		resumeText = extractResumeText(fileBytes)
	} else {
		var body struct {
			ResumeText string `json:"resume_text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, apperrors.BadRequest("invalid JSON body"))
			return
		}
		resumeText = body.ResumeText
	}

	if strings.TrimSpace(resumeText) == "" {
		writeError(w, apperrors.BadRequest("could not read any text from the resume — try pasting the text instead"))
		return
	}

	writeSuccess(w, http.StatusOK, resume.ParseText(resumeText))
}
