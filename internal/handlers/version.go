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

type GetVersionSuccessResponse struct {
	Name        string `example:"schoolbox-backend"`
	Version     string `example:"1.0.0"`
	Environment string `example:"development"`
}

// GetVersion
//
// @Summary		Get the api version
// @Description	Get current api name, version and deployment env (prod, dev)
// @Tags			version
// @Accept			json
// @Produce		json
// @Success		200	{object} apiResponses.BaseResponse{data=GetVersionSuccessResponse}
// @Router 			/v		[get]
func (h *VersionHandler) GetVersion(w http.ResponseWriter, r *http.Request) {
	if err := gecho.Handlers.HandleMethod(w, r, http.MethodGet); err != nil {
		err.Send() // Automatically sends 405 Method Not Allowed
		return
	}

	versionInfo := GetVersionSuccessResponse{
		Name:        h.config.App.Name,
		Version:     h.config.App.Version,
		Environment: h.config.App.Environment,
	}

	gecho.Success(w).WithData(versionInfo).Send()
}
