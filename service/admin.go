package service

import (
	"context"
	"strings"

	"github.com/helloamitkr/cvfit-backend/model"
	"github.com/helloamitkr/cvfit-tools/auth"
	apperrors "github.com/helloamitkr/cvfit-tools/errors"
)

// IsAdmin returns true if the user is an admin — either via the is_admin flag in
// DynamoDB, or because their email is in the ADMIN_EMAILS config allowlist.
func (s *Service) IsAdmin(ctx context.Context, userID string) bool {
	if userID == "" {
		return false
	}
	u, err := s.GetByID(ctx, userID)
	if err != nil || u == nil {
		return false
	}
	return u.IsAdmin || s.isAdminEmail(u.Email)
}

// isAdminEmail reports whether an email is in the code/env-configured admin allowlist.
func (s *Service) isAdminEmail(email string) bool {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return false
	}
	for _, e := range s.AdminEmails {
		if strings.ToLower(strings.TrimSpace(e)) == email {
			return true
		}
	}
	return false
}

// applyAdminEmail flags a profile as admin when its email is in the allowlist, so
// the frontend shows the Admin link right after login (not only on a DB flag).
func (s *Service) applyAdminEmail(p *auth.UserProfile) *auth.UserProfile {
	if p != nil && !p.IsAdmin && s.isAdminEmail(p.Email) {
		p.IsAdmin = true
	}
	return p
}

// AdminStats holds the aggregate numbers shown on the admin dashboard.
type AdminStats struct {
	TotalUsers        int `json:"total_users"`
	TotalOrders       int `json:"total_orders"`
	SuccessfulOrders  int `json:"successful_orders"`
	PendingOrders     int `json:"pending_orders"`
	FailedOrders      int `json:"failed_orders"`
	TotalRevenuePaise int `json:"total_revenue_paise"`
}

// ListUsers returns all users in the users table.
func (s *Service) ListUsers(ctx context.Context) ([]*auth.User, *apperrors.AppError) {
	var users []*auth.User
	if err := s.Dynamo.Scan(ctx, s.UsersTableName, &users); err != nil {
		return nil, apperrors.InternalServerWrap("failed to list users", err)
	}
	return users, nil
}

// ListOrders returns all orders in the orders table.
func (s *Service) ListOrders(ctx context.Context) ([]*model.Order, *apperrors.AppError) {
	var orders []*model.Order
	if err := s.Dynamo.Scan(ctx, s.OrdersTableName, &orders); err != nil {
		return nil, apperrors.InternalServerWrap("failed to list orders", err)
	}
	return orders, nil
}

// GetAdminStats computes aggregate stats from users and orders tables.
func (s *Service) GetAdminStats(ctx context.Context) (*AdminStats, *apperrors.AppError) {
	users, err := s.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	orders, err := s.ListOrders(ctx)
	if err != nil {
		return nil, err
	}

	stats := &AdminStats{TotalUsers: len(users), TotalOrders: len(orders)}
	for _, o := range orders {
		switch o.Status {
		case "success":
			stats.SuccessfulOrders++
			stats.TotalRevenuePaise += o.Amount
		case "pending":
			stats.PendingOrders++
		case "failed":
			stats.FailedOrders++
		}
	}
	return stats, nil
}
