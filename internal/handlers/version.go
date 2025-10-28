package handlers

import (
	"net/http"

	"github.com/CLDWare/schoolbox-backend/config"
	"github.com/MonkyMars/gecho"
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

// GetVersion handles GET /v requests
func (h *VersionHandler) GetVersion(w http.ResponseWriter, r *http.Request) {
	if err := gecho.Handlers.HandleMethod(w, r, http.MethodGet); err != nil {
		err.Send() // Automatically sends 405 Method Not Allowed
		return
	}

	versionInfo := map[string]string{
		"name":        h.config.App.Name,
		"version":     h.config.App.Version,
		"environment": h.config.App.Environment,
	}

	gecho.Success(w).WithData(versionInfo).Send()
}
