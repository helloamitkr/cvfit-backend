package handlers

import (
	"net/http"
	"net/url"
	"strconv"
	"time"
)

func (d *Deps) HandleOAuthBegin(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	authURL, appErr := d.Svc.BeginOAuth(provider)
	if appErr != nil {
		writeError(w, appErr)
		return
	}
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

func (d *Deps) HandleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")

	if errParam := r.URL.Query().Get("error"); errParam != "" {
		http.Redirect(w, r, d.Svc.FrontendURL+"/login?error="+url.QueryEscape(errParam), http.StatusTemporaryRedirect)
		return
	}

	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")

	profile, token, appErr := d.Svc.CompleteOAuth(r.Context(), provider, state, code)
	if appErr != nil {
		http.Redirect(w, r, d.Svc.FrontendURL+"/login?error="+url.QueryEscape(appErr.Message), http.StatusTemporaryRedirect)
		return
	}

	q := url.Values{}
	q.Set("token", token)
	q.Set("id", profile.ID)
	q.Set("email", profile.Email)
	q.Set("name", profile.Name)
	q.Set("is_admin", strconv.FormatBool(profile.IsAdmin))
	q.Set("created_at", profile.CreatedAt.Format(time.RFC3339))
	http.Redirect(w, r, d.Svc.FrontendURL+"/auth/callback?"+q.Encode(), http.StatusTemporaryRedirect)
}
