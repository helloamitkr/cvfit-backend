package service

import (
	"context"
	"os"
	"strconv"
	"time"
)

const defaultFreeAIPerDay = 3

// FreeAIPerDay is the number of free AI generations a signed-in user gets each day
// before needing to unlock unlimited usage. Overridable via FREE_AI_PER_DAY.
func (s *Service) FreeAIPerDay() int {
	if v := os.Getenv("FREE_AI_PER_DAY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
	}
	return defaultFreeAIPerDay
}

func aiToday() string { return time.Now().UTC().Format("2006-01-02") }

// CheckAIQuota reports whether the user may make another AI generation right now.
// A user who purchased an unlock today has unlimited usage. This is read-only
// (it does not consume a credit) and fails open on DB errors so users are never
// blocked by an infrastructure hiccup.
func (s *Service) CheckAIQuota(ctx context.Context, userID string) (allowed bool, used, limit int) {
	limit = s.FreeAIPerDay()
	u, err := s.GetByID(ctx, userID)
	if err != nil || u == nil {
		return true, 0, limit // fail open
	}
	if u.AIPaidDate == aiToday() {
		return true, 0, limit // paid today → unlimited
	}
	if u.AIUsageDate != aiToday() {
		return true, 0, limit // new day → counter has reset
	}
	return u.AIUsageCount < limit, u.AIUsageCount, limit
}

// ConsumeAIQuota records one AI generation against the user's daily allowance.
// It is a no-op for users who unlocked unlimited today, and best-effort otherwise
// (errors are ignored — we never fail a successful generation over bookkeeping).
func (s *Service) ConsumeAIQuota(ctx context.Context, userID string) {
	u, err := s.GetByID(ctx, userID)
	if err != nil || u == nil {
		return
	}
	if u.AIPaidDate == aiToday() {
		return // unlimited — nothing to count
	}
	if u.AIUsageDate != aiToday() {
		u.AIUsageDate = aiToday()
		u.AIUsageCount = 0
	}
	u.AIUsageCount++
	_ = s.Save(ctx, u)
}

// GrantAIUnlock marks the user as having unlimited AI for the current day.
// Called after a successful ₹10 payment so the same purchase that removes the
// PDF watermark also lifts the AI limit.
func (s *Service) GrantAIUnlock(ctx context.Context, userID string) {
	u, err := s.GetByID(ctx, userID)
	if err != nil || u == nil {
		return
	}
	u.AIPaidDate = aiToday()
	_ = s.Save(ctx, u)
}
