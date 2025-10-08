package handlers

import (
	"net/http"

	"github.com/CLDWare/schoolbox-backend/config"
	"github.com/CLDWare/schoolbox-backend/pkg/response"
)

// VersionHandler handles version-related requests
type VersionHandler struct {
	config *config.Config
}

// NewVersionHandler creates a new version handler
func NewVersionHandler(cfg *config.Config) *VersionHandler {
	return &VersionHandler{
		config: cfg,
	}
}

// VersionResponse represents the version information
type VersionResponse struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Environment string `json:"environment"`
}

// GetVersion handles GET /v requests
func (h *VersionHandler) GetVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	versionInfo := VersionResponse{
		Name:        h.config.App.Name,
		Version:     h.config.App.Version,
		Environment: h.config.App.Environment,
	}

	response.Success(w, versionInfo)
}
