package handlers

import "github.com/helloamitkr/cvfit-backend/service"

// Deps is the dependency container for the HTTP handler layer.
type Deps struct {
	Svc *service.Service
}
