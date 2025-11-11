package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/CLDWare/schoolbox-backend/config"
	"github.com/CLDWare/schoolbox-backend/pkg/logger"
	"github.com/MonkyMars/gecho"
)

// RegistrationHandler handles registration-related requests
type RegistrationHandler struct {
	config           *config.Config
	websocketHandler *WebsocketHandler
}

// NewRegistrationHandler creates a new registration handler
func NewRegistrationHandler(cfg *config.Config, websocketHandler *WebsocketHandler) *RegistrationHandler {
	return &RegistrationHandler{
		config:           cfg,
		websocketHandler: websocketHandler,
	}
}

type RegistrationPinBody struct {
	Pin uint `json:"pin"`
}

// handles POST /registration_pin requests
func (h *RegistrationHandler) PostRegistrationPin(w http.ResponseWriter, r *http.Request) {
	if err := gecho.Handlers.HandleMethod(w, r, http.MethodPost); err != nil {
		err.Send() // Automatically sends 405 Method Not Allowed
		return
	}
	var body RegistrationPinBody

	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		gecho.BadRequest(w).WithMessage(err.Error()).Send()
		logger.Err(err)
		return
	}
	device, err := h.websocketHandler.registerWithPin(body.Pin)
	if err != nil {
		if err.Error() == "No connectionID for this pin" {
			gecho.BadRequest(w).WithMessage("Invalid pin").Send()
		} else {
			gecho.InternalServerError(w).WithMessage(err.Error()).Send()
		}
		return
	}

	RegistrationPinData := map[string]any{
		"id": device.ID,
	}

	gecho.Created(w).WithData(RegistrationPinData).Send()
}
