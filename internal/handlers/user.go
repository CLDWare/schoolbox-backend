package handlers

import (
	"net/http"

	"github.com/CLDWare/schoolbox-backend/config"
	contextkeys "github.com/CLDWare/schoolbox-backend/internal/contextKeys"
	models "github.com/CLDWare/schoolbox-backend/pkg/db"
	"github.com/MonkyMars/gecho"
	"gorm.io/gorm"
)

// UserHandler handles requests about users
type UserHandler struct {
	config *config.Config
	db     *gorm.DB
}

// NewUserHandler creates a new user handler
func NewUserHandler(cfg *config.Config, db *gorm.DB) *UserHandler {
	return &UserHandler{
		config: cfg,
		db:     db,
	}
}

// handles GET /me requests
func (h *UserHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	if err := gecho.Handlers.HandleMethod(w, r, http.MethodGet); err != nil {
		err.Send() // Automatically sends 405 Method Not Allowed
		return
	}

	ctx := r.Context()
	user, ok := ctx.Value(contextkeys.AuthUserKey).(models.User)
	if !ok {
		gecho.InternalServerError(w).Send()
	}

	userInfo := map[string]any{
		"email":            user.Email,
		"google_sub":       user.GoogleSubject,
		"name":             user.Name,
		"display_name":     user.DisplayName,
		"role":             user.Role,
		"default_question": user.DefaultQuestion,
	}

	gecho.Success(w).WithData(userInfo).Send()
}
