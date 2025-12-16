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
	DeviceHandler         *handlers.DeviceHandler
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
		SessionHandler:        handlers.NewSessionHandler(cfg, db, websocketHandler),
		DeviceHandler:         handlers.NewDeviceHandler(cfg, db),
	}
}

func NewMethodRouter(handlerFuncMap map[string]http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if handlerFuncMap[r.Method] != nil {
			handlerFuncMap[r.Method](w, r)
		} else {
			gecho.MethodNotAllowed(w).Send()
		}
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

	auth := middleware.AuthenticationMiddleware{
		DB: api.database,
	}
	// User api
	mux.HandleFunc("/me", auth.Required(api.UserHandler.GetMe))
	mux.HandleFunc("/user", auth.RequiresAdmin(api.UserHandler.GetUser))
	mux.HandleFunc("/user/{id}", auth.RequiresAdmin(api.UserHandler.GetUserById))

	// Device api
	mux.HandleFunc("/device", auth.RequiresAdmin(api.DeviceHandler.GetDevice))
	mux.HandleFunc("/device/{id}", auth.RequiresAdmin(api.DeviceHandler.GetDeviceById))

	// Session api
	sessionRouter := NewMethodRouter(map[string]http.HandlerFunc{
		http.MethodGet:  api.SessionHandler.GetSession,
		http.MethodPost: api.SessionHandler.PostSession,
	})
	mux.HandleFunc("/session", auth.Required(sessionRouter))
	mux.HandleFunc("/session/stop", auth.Required(api.SessionHandler.PostSessionStop))
	mux.HandleFunc("/session/current", auth.Required(api.SessionHandler.GetCurrentSession))
	mux.HandleFunc("/session/{id}", auth.Required(api.SessionHandler.GetSessionById))

	mux.HandleFunc("/registration_pin", auth.RequiresAdmin(api.registrationHandler.PostRegistrationPin))

	// Fallback route - must be last because it matches all routes.
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
