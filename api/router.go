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
	database              *gorm.DB
	versionHandler        *handlers.VersionHandler
	websocketHandler      *handlers.WebsocketHandler
	registrationHandler   *handlers.RegistrationHandler
	authenticationHandler *handlers.AuthenticationHandler
	UserHandler           *handlers.UserHandler
	SessionHandler        *handlers.SessionHandler
}

// NewAPI creates a new API instance
func NewAPI(db *gorm.DB) *API {
	cfg := config.Get()
	websocketHandler := handlers.NewWebsocketHandler(cfg, db)
	return &API{
		database:              db,
		versionHandler:        handlers.NewVersionHandler(cfg),
		websocketHandler:      websocketHandler,
		registrationHandler:   handlers.NewRegistrationHandler(cfg, websocketHandler),
		authenticationHandler: handlers.NewAuthenticationHandler(cfg, db),
		UserHandler:           handlers.NewUserHandler(cfg, db),
		SessionHandler:        handlers.NewSessionHandler(cfg, db),
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

	// Frontend authentication
	mux.HandleFunc("/login", api.authenticationHandler.GetLogin)                  // redirect to google OAuth consent
	mux.HandleFunc("/oauth2callback", api.authenticationHandler.GetOAuthCallback) // google OAuth consent callback

	// user api
	auth := middleware.AuthenticationMiddleware{
		DB: api.database,
	}
	mux.HandleFunc("/me", auth.Required(api.UserHandler.GetMe))
	mux.HandleFunc("/user", auth.RequiresAdmin(api.UserHandler.GetUser))
	mux.HandleFunc("/user/{id}", auth.RequiresAdmin(api.UserHandler.GetUserById))

	mux.HandleFunc("/session", auth.Required(api.SessionHandler.GetSession))
	mux.HandleFunc("/session/{id}", auth.Required(api.SessionHandler.GetSessionById))

	mux.HandleFunc("/registration_pin", auth.Required(api.registrationHandler.PostRegistrationPin))

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
