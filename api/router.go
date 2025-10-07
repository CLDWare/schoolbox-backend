package api

import (
	"net/http"

	"github.com/CLDWare/schoolbox-backend/internal/middleware"
	"github.com/CLDWare/schoolbox-backend/pkg/response"
)

// API holds the API dependencies
type API struct {
	// userHandler *handlers.UserHandler for example
}

// NewAPI creates a new API instance
func NewAPI() *API {
	return &API{}
}

// CreateMux creates and configures the HTTP mux
func (api *API) CreateMux() *http.ServeMux {
	mux := http.NewServeMux()
	api.setupRoutes(mux)
	return mux
}

// setupRoutes configures all the routes.
func (api *API) setupRoutes(mux *http.ServeMux) {
	// fallback route - must be last because it matches all routes.
	mux.HandleFunc("/", fallBack)
}

// ApplyMiddleware applies middleware to a handler
func ApplyMiddleware(handler http.Handler) http.Handler {
	return middleware.LoggingMiddleware(
		middleware.CORSMiddleware(handler),
	)
}

func fallBack(w http.ResponseWriter, r *http.Request) {
	response.Error(w, http.StatusNotFound, "Route not found")
}
