package api

import (
	"net/http"

	"github.com/MonkyMars/gecho"
	"gorm.io/gorm"

	"github.com/CLDWare/schoolbox-backend/config"
	"github.com/CLDWare/schoolbox-backend/internal/handlers"
	"github.com/CLDWare/schoolbox-backend/internal/middleware"
)

// API holds the API dependencies
type API struct {
	versionHandler      *handlers.VersionHandler
	websocketHandler    *handlers.WebsocketHandler
	registrationHandler *handlers.RegistrationHandler
}

// NewAPI creates a new API instance
func NewAPI(db *gorm.DB) *API {
	cfg := config.Get()
	websocketHandler := handlers.NewWebsocketHandler(cfg, db)
	return &API{
		versionHandler:      handlers.NewVersionHandler(cfg),
		websocketHandler:    websocketHandler,
		registrationHandler: handlers.NewRegistrationHandler(cfg, websocketHandler),
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
	// Websocket connection
	mux.HandleFunc("/ws", api.websocketHandler.InitialiseWebsocket)
	//
	mux.HandleFunc("/registration_pin", api.registrationHandler.PostRegistrationPin)

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
