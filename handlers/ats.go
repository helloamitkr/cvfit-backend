package handlers

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/helloamitkr/cvfit-backend/service"
	apperrors "github.com/helloamitkr/cvfit-tools/errors"
	"github.com/helloamitkr/cvfit-tools/resume"
)

func (d *Deps) HandleATSScore(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		writeError(w, apperrors.BadRequest("failed to parse form: "+err.Error()))
		return
	}

	file, header, err := r.FormFile("resume")
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

	result, appErr := service.ScoreATS(r.Context(), fileBytes, header.Filename, r.FormValue("job_description"))
	if appErr != nil {
		writeError(w, appErr)
		return
	}
	writeSuccess(w, http.StatusOK, result)
}

func (d *Deps) HandleATSScoreResume(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Resume         resume.Resume `json:"resume"`
		JobDescription string        `json:"job_description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, apperrors.BadRequest("invalid JSON body"))
		return
	}
	result := service.ScoreATSFromResume(body.Resume, body.JobDescription)
	writeSuccess(w, http.StatusOK, result)
}
