package service

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/helloamitkr/cvfit-tools/auth"
	apperrors "github.com/helloamitkr/cvfit-tools/errors"
	"github.com/helloamitkr/cvfit-tools/resume"
)

// AuthClaims re-exports auth.AuthClaims so callers (e.g. middleware) don't need a new import.
type AuthClaims = auth.AuthClaims

// ── auth.UserStore implementation ─────────────────────────────────────────────

func (s *Service) GetByEmail(ctx context.Context, email string) (*auth.User, error) {
	var users []auth.User
	err := s.Dynamo.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(s.UsersTableName),
		IndexName:              aws.String("email-index"),
		KeyConditionExpression: aws.String("email = :e"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":e": &types.AttributeValueMemberS{Value: email},
		},
	}, &users)
	if err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return nil, nil
	}
	return &users[0], nil
}

func (s *Service) GetByID(ctx context.Context, id string) (*auth.User, error) {
	key := map[string]types.AttributeValue{
		"id": &types.AttributeValueMemberS{Value: id},
	}
	var u auth.User
	if err := s.Dynamo.GetItem(ctx, s.UsersTableName, key, &u); err != nil {
		return nil, err
	}
	if u.ID == "" {
		return nil, nil
	}
	return &u, nil
}

func (s *Service) Save(ctx context.Context, u *auth.User) error {
	return s.Dynamo.PutItem(ctx, s.UsersTableName, u)
}

// ── Auth wrappers ──────────────────────────────────────────────────────────────

func (s *Service) Register(ctx context.Context, email, password, name string) (*auth.UserProfile, string, *apperrors.AppError) {
	p, tok, err := auth.Register(ctx, s, s.JWTSecret, email, password, name)
	return s.applyAdminEmail(p), tok, err
}

func (s *Service) Login(ctx context.Context, email, password string) (*auth.UserProfile, string, *apperrors.AppError) {
	p, tok, err := auth.Login(ctx, s, s.JWTSecret, email, password)
	return s.applyAdminEmail(p), tok, err
}

func (s *Service) Me(ctx context.Context, userID string) (*auth.UserProfile, *apperrors.AppError) {
	u, err := s.GetByID(ctx, userID)
	if err != nil {
		return nil, apperrors.InternalServerWrap("failed to fetch user", err)
	}
	if u == nil {
		return nil, apperrors.NotFound("user not found")
	}
	return s.applyAdminEmail(u.Profile()), nil
}

func (s *Service) ParseToken(tokenStr string) (*AuthClaims, error) {
	return auth.ParseToken(tokenStr, s.JWTSecret)
}

func (s *Service) RequestOTP(ctx context.Context, email, name, password string) *apperrors.AppError {
	if s.Email == nil {
		return apperrors.New(503, "email service not configured")
	}
	return auth.RequestOTP(ctx, s, s.Email, email, name, password)
}

func (s *Service) VerifyOTP(ctx context.Context, email, code string) (*auth.UserProfile, string, *apperrors.AppError) {
	p, tok, err := auth.VerifyOTP(ctx, s, s.Email, s.JWTSecret, email, code)
	return s.applyAdminEmail(p), tok, err
}

// ── Profile ────────────────────────────────────────────────────────────────────

type UpdateProfileInput struct {
	FirstName      string                 `json:"first_name"`
	LastName       string                 `json:"last_name"`
	Headline       string                 `json:"headline"`
	Phone          string                 `json:"phone"`
	Location       string                 `json:"location"`
	Website        string                 `json:"website"`
	LinkedIn       string                 `json:"linkedin"`
	GitHub         string                 `json:"github"`
	PhotoURL       string                 `json:"photo_url"`
	Certifications []resume.Certification `json:"certifications"`
	Education      []resume.Education     `json:"education"`
	Experience     []resume.Experience    `json:"experience"`
	Summary        string                 `json:"summary"`
	Skills         []resume.Skill         `json:"skills"`
	Projects       []resume.Project       `json:"projects"`
}

func (s *Service) UpdateProfile(ctx context.Context, userID string, in UpdateProfileInput) (*auth.User, error) {
	u, err := s.GetByID(ctx, userID)
	if err != nil || u == nil {
		return nil, err
	}
	u.FirstName = in.FirstName
	u.LastName = in.LastName
	u.Headline = in.Headline
	u.Phone = in.Phone
	u.Location = in.Location
	u.Website = in.Website
	u.LinkedIn = in.LinkedIn
	u.GitHub = in.GitHub
	u.PhotoURL = in.PhotoURL
	u.Certifications = in.Certifications
	u.Education = in.Education
	u.Experience = in.Experience
	u.Summary = in.Summary
	u.Skills = in.Skills
	u.Projects = in.Projects
	return u, s.Save(ctx, u)
}
