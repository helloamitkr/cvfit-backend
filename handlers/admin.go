package handlers

import (
	"encoding/json"
	"net/http"

	apperrors "github.com/helloamitkr/cvfit-tools/errors"
)

// HandleAdminMe returns the current user's email and admin status.
// GET /api/admin/me  (RequireAuth only — useful for debugging)
func (d *Deps) HandleAdminMe(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromCtx(r.Context())
	u, err := d.Svc.GetByID(r.Context(), userID)
	if err != nil || u == nil {
		writeError(w, apperrors.NotFound("user not found"))
		return
	}
	writeSuccess(w, http.StatusOK, map[string]interface{}{
		"id":       u.ID,
		"email":    u.Email,
		"is_admin": d.Svc.IsAdmin(r.Context(), userID),
	})
}

// HandleAdminStats returns aggregate platform stats.
// GET /api/admin/stats
func (d *Deps) HandleAdminStats(w http.ResponseWriter, r *http.Request) {
	stats, appErr := d.Svc.GetAdminStats(r.Context())
	if appErr != nil {
		writeError(w, appErr)
		return
	}
	writeSuccess(w, http.StatusOK, stats)
}

// HandleAdminUsers returns all users.
// GET /api/admin/users
func (d *Deps) HandleAdminUsers(w http.ResponseWriter, r *http.Request) {
	users, appErr := d.Svc.ListUsers(r.Context())
	if appErr != nil {
		writeError(w, appErr)
		return
	}
	// Strip password hashes before sending.
	type safeUser struct {
		ID        string `json:"id"`
		Email     string `json:"email"`
		Name      string `json:"name"`
		Provider  string `json:"provider"`
		CreatedAt string `json:"created_at"`
	}
	out := make([]safeUser, 0, len(users))
	for _, u := range users {
		out = append(out, safeUser{
			ID:        u.ID,
			Email:     u.Email,
			Name:      u.Name,
			Provider:  u.Provider,
			CreatedAt: u.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}
	writeSuccess(w, http.StatusOK, out)
}

// HandleAdminOrders returns all orders.
// GET /api/admin/orders
func (d *Deps) HandleAdminOrders(w http.ResponseWriter, r *http.Request) {
	orders, appErr := d.Svc.ListOrders(r.Context())
	if appErr != nil {
		writeError(w, appErr)
		return
	}
	writeSuccess(w, http.StatusOK, orders)
}

// HandleOrderFailed marks an order as failed (called by frontend on Razorpay modal dismiss).
// POST /api/payment/cancel
// Body: { "order_id": "..." }
func (d *Deps) HandleOrderFailed(w http.ResponseWriter, r *http.Request) {
	var body struct {
		OrderID string `json:"order_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.OrderID == "" {
		writeError(w, apperrors.BadRequest("order_id is required"))
		return
	}
	d.Svc.OrderStatusFailed(r.Context(), body.OrderID)
	writeSuccess(w, http.StatusOK, map[string]bool{"ok": true})
}
