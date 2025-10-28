package api

import (
	"net/http"

	"github.com/MonkyMars/gecho"

	"github.com/CLDWare/schoolbox-backend/config"
	"github.com/CLDWare/schoolbox-backend/internal/handlers"
	"github.com/CLDWare/schoolbox-backend/internal/middleware"
)

// API holds the API dependencies
type API struct {
	versionHandler *handlers.VersionHandler
}

// NewAPI creates a new API instance
func NewAPI() *API {
	cfg := config.Get()
	return &API{
		versionHandler: handlers.NewVersionHandler(cfg),
	}
}

// CreateMux creates and configures the HTTP mux
func (api *API) CreateMux() *http.ServeMux {
	mux := http.NewServeMux()
	api.setupRoutes(mux)
	return mux
}

// setupRoutes configures all the routes.
func (api *API) setupRoutes(mux *http.ServeMux) {
	// Version route
	mux.HandleFunc("/v", api.versionHandler.GetVersion)

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
	gecho.NotFound(w).Send()
}
