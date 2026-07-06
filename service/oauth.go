package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"

	"github.com/helloamitkr/cvfit-tools/auth"
	apperrors "github.com/helloamitkr/cvfit-tools/errors"
)

// ── State store ───────────────────────────────────────────────────────────────

type oauthStateStore struct {
	mu     sync.Mutex
	states map[string]time.Time
}

var globalOAuthStates = &oauthStateStore{states: make(map[string]time.Time)}

func (s *oauthStateStore) generate() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	token := hex.EncodeToString(b)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.states[token] = time.Now().Add(10 * time.Minute)
	for k, exp := range s.states {
		if time.Now().After(exp) {
			delete(s.states, k)
		}
	}
	return token
}

func (s *oauthStateStore) validate(token string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, ok := s.states[token]
	if !ok {
		return false
	}
	delete(s.states, token)
	return time.Now().Before(exp)
}

// ── OAuth config ──────────────────────────────────────────────────────────────

func buildOAuthConfig(provider string, pc OAuthProviderConfig) (*oauth2.Config, error) {
	redirectURL := fmt.Sprintf("%s/api/auth/oauth/%s/callback", pc.RedirectBase, provider)
	switch provider {
	case "google":
		return &oauth2.Config{
			ClientID:     pc.ClientID,
			ClientSecret: pc.ClientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"openid", "profile", "email"},
			Endpoint:     google.Endpoint,
		}, nil
	case "github":
		return &oauth2.Config{
			ClientID:     pc.ClientID,
			ClientSecret: pc.ClientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"read:user", "user:email"},
			Endpoint:     github.Endpoint,
		}, nil
	case "linkedin":
		return &oauth2.Config{
			ClientID:     pc.ClientID,
			ClientSecret: pc.ClientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"openid", "profile", "email"},
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://www.linkedin.com/oauth/v2/authorization",
				TokenURL: "https://www.linkedin.com/oauth/v2/accessToken",
			},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported OAuth provider: %s", provider)
	}
}

// ── Public API ────────────────────────────────────────────────────────────────

func (s *Service) BeginOAuth(provider string) (string, *apperrors.AppError) {
	pc, ok := s.OAuthProviders[provider]
	if !ok {
		return "", apperrors.BadRequest("unsupported OAuth provider: " + provider)
	}
	cfg, err := buildOAuthConfig(provider, pc)
	if err != nil {
		return "", apperrors.BadRequest(err.Error())
	}
	state := globalOAuthStates.generate()
	return cfg.AuthCodeURL(state, oauth2.AccessTypeOnline), nil
}

func (s *Service) CompleteOAuth(ctx context.Context, provider, state, code string) (*auth.UserProfile, string, *apperrors.AppError) {
	if !globalOAuthStates.validate(state) {
		return nil, "", apperrors.New(400, "invalid or expired OAuth state — please try signing in again")
	}

	pc, ok := s.OAuthProviders[provider]
	if !ok {
		return nil, "", apperrors.BadRequest("unsupported OAuth provider: " + provider)
	}
	cfg, err := buildOAuthConfig(provider, pc)
	if err != nil {
		return nil, "", apperrors.BadRequest(err.Error())
	}

	oauthToken, err := cfg.Exchange(ctx, code)
	if err != nil {
		return nil, "", apperrors.InternalServerWrap("failed to exchange OAuth code", err)
	}

	info, err := fetchOAuthUserInfo(ctx, cfg, provider, oauthToken)
	if err != nil {
		return nil, "", apperrors.InternalServerWrap("failed to fetch user info from "+provider, err)
	}

	u, err := s.upsertOAuthUser(ctx, info.email, info.name, provider)
	if err != nil {
		return nil, "", apperrors.InternalServerWrap("failed to upsert OAuth user", err)
	}

	jwtToken, err := auth.SignToken(u, s.JWTSecret)
	if err != nil {
		return nil, "", apperrors.InternalServerWrap("failed to sign token", err)
	}

	return s.applyAdminEmail(u.Profile()), jwtToken, nil
}

// ── OAuth user upsert ─────────────────────────────────────────────────────────

func (s *Service) upsertOAuthUser(ctx context.Context, email, name, provider string) (*auth.User, error) {
	existing, err := s.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
	return s.createOAuthUser(ctx, email, name, provider)
}

func (s *Service) createOAuthUser(ctx context.Context, email, name, provider string) (*auth.User, error) {
	u := &auth.User{
		ID:         uuid.New().String(),
		Email:      email,
		Name:       name,
		Provider:   provider,
		IsVerified: true,
		CreatedAt:  time.Now().UTC(),
	}
	return u, s.Save(ctx, u)
}

// ── User info fetching ────────────────────────────────────────────────────────

type oauthUserInfo struct {
	email string
	name  string
}

func fetchOAuthUserInfo(ctx context.Context, cfg *oauth2.Config, provider string, token *oauth2.Token) (*oauthUserInfo, error) {
	httpClient := cfg.Client(ctx, token)

	var userInfoURL string
	switch provider {
	case "google":
		userInfoURL = "https://www.googleapis.com/oauth2/v3/userinfo"
	case "github":
		userInfoURL = "https://api.github.com/user"
	case "linkedin":
		userInfoURL = "https://api.linkedin.com/v2/userinfo"
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}

	resp, err := httpClient.Get(userInfoURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var raw struct {
		Email string `json:"email"`
		Name  string `json:"name"`
		Login string `json:"login"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse userinfo from %s: %w", provider, err)
	}

	name := raw.Name
	if name == "" {
		name = raw.Login
	}
	if name == "" && raw.Email != "" {
		name = strings.Split(raw.Email, "@")[0]
	}

	email := raw.Email
	if provider == "github" && email == "" {
		email, err = fetchGitHubPrimaryEmail(httpClient)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve GitHub email: %w", err)
		}
	}

	if email == "" {
		return nil, fmt.Errorf("no email returned by %s — ensure your account has a verified email", provider)
	}

	return &oauthUserInfo{email: email, name: name}, nil
}

func fetchGitHubPrimaryEmail(client *http.Client) (string, error) {
	resp, err := client.Get("https://api.github.com/user/emails")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", err
	}
	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}
	for _, e := range emails {
		if e.Verified {
			return e.Email, nil
		}
	}
	return "", fmt.Errorf("no verified email found on GitHub account")
}
