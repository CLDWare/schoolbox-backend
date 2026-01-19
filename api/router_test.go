package api

import (
	"net/http"
	"net/http/httptest"
	"os"
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

	// Create API instance
	api := NewAPI(db)
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
