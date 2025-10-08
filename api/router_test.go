package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CLDWare/schoolbox-backend/pkg/logger"
)

func TestAPI_WithMiddleware(t *testing.T) {
	// Initialize logger for middleware test
	logger.Init()

	// Create API instance
	api := NewAPI()
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
