package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"syscall"
	"testing"

	models "github.com/CLDWare/schoolbox-backend/pkg/db"
	"github.com/CLDWare/schoolbox-backend/pkg/logger"
)

func TestAPI_WithMiddleware(t *testing.T) {
	// Initialize logger for middleware test
	logger.Init()

	// Initialise Database
	db, err := models.InitialiseDatabase()
	if err != nil {
		logger.Err(err)
		os.Exit(1)
	}

	// Create interrupt signal to gracefully shutdown the server (well, we do it because the NewAPI function needs it)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Create API instance
	api := NewAPI(db, quit)
	mux := api.CreateMux()
	handler := ApplyMiddleware(mux)

	req := httptest.NewRequest(http.MethodGet, "/v", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Check that the request went through middleware and reached the handler
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Check CORS headers are present (from CORSMiddleware)
	corsHeader := w.Header().Get("Access-Control-Allow-Origin")
	if corsHeader == "" {
		t.Error("expected CORS headers to be set by middleware")
	}
}
