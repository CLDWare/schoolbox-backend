package handlers

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"

	"github.com/CLDWare/schoolbox-backend/config"
	contextkeys "github.com/CLDWare/schoolbox-backend/internal/contextKeys"
	models "github.com/CLDWare/schoolbox-backend/pkg/db"
	"github.com/CLDWare/schoolbox-backend/pkg/logger"
	"github.com/MonkyMars/gecho"
	"gorm.io/gorm"
)

// UserHandler handles requests about users
type UserHandler struct {
	config *config.Config
	db     *gorm.DB
}

// NewUserHandler creates a new UserHandler
func NewUserHandler(cfg *config.Config, db *gorm.DB) *UserHandler {
	return &UserHandler{
		config: cfg,
		db:     db,
	}
}

func toUserInfo(user models.User) map[string]any {
	return map[string]any{
		"id":               user.ID,
		"email":            user.Email,
		"google_sub":       user.GoogleSubject,
		"role":             user.Role,
		"joinedAt":         user.CreatedAt,
		"name":             user.Name,
		"display_name":     user.DisplayName,
		"default_question": user.DefaultQuestion,
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

	userInfo := toUserInfo(user)

	gecho.Success(w).WithData(userInfo).Send()
}

// handles GET /user requests
func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	if err := gecho.Handlers.HandleMethod(w, r, http.MethodGet); err != nil {
		err.Send() // Automatically sends 405 Method Not Allowed
		return
	}

	query := r.URL.Query()
	dbQuery := h.db.Model(&models.User{})

	// return count filters
	if limitStr := query.Get("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil {
			gecho.BadRequest(w).WithMessage(err.Error()).Send()
			return
		}
		if limit > 20 {
			limit = 20
		}
		dbQuery = dbQuery.Limit(limit)
	} else {
		dbQuery = dbQuery.Limit(20)
	}
	if offsetStr := query.Get("offset"); offsetStr != "" {
		offset, err := strconv.Atoi(offsetStr)
		if err != nil {
			gecho.BadRequest(w).WithMessage(err.Error()).Send()
			return
		}
		dbQuery = dbQuery.Offset(offset)
	}
	// filters
	if roleStr := query.Get("role"); roleStr != "" {
		role, err := strconv.ParseUint(roleStr, 10, 0)
		if err != nil {
			gecho.BadRequest(w).WithMessage(err.Error()).Send()
			return
		}
		dbQuery = dbQuery.Where("role = ?", role)
	}

	var users []models.User
	err := dbQuery.Find(&users).Error
	if err != nil {
		gecho.InternalServerError(w).Send()
		logger.Err(err.Error())
		return
	}

	userInfoArray := []map[string]any{}
	for _, user := range users {
		userInfoArray = append(userInfoArray, toUserInfo(user))
	}

	gecho.Success(w).WithData(userInfoArray).Send()
}

// handles GET /user/{id} requests
func (h *UserHandler) GetUserById(w http.ResponseWriter, r *http.Request) {
	if err := gecho.Handlers.HandleMethod(w, r, http.MethodGet); err != nil {
		err.Send() // Automatically sends 405 Method Not Allowed
		return
	}

	query := r.URL.Query()
	dbQuery := h.db.Model(&models.User{})

	idStr := r.PathValue("id")
	idType := query.Get("type")
	if idType == "" {
		idType = "id"
	}

	switch idType {
	case "id":
		userID, err := strconv.ParseUint(idStr, 10, 0)
		if err != nil {
			gecho.BadRequest(w).WithMessage("Invalid user ID, expected positive integer").Send()
			return
		}
		dbQuery = dbQuery.Where("id = ?", userID)
	case "email":
		ok, err := regexp.Match(`^[\w\-\.]+@([\w-]+\.)+[\w-]{2,}$`, []byte(idStr))
		if !ok {
			gecho.BadRequest(w).WithMessage(fmt.Sprintf("Invalid email '%s'", idStr)).Send()
			return
		}
		if err != nil {
			gecho.InternalServerError(w).WithMessage(err.Error()).Send()
			return
		}
		dbQuery = dbQuery.Where("email = ?", idStr)
	default:
		gecho.BadRequest(w).WithMessage(fmt.Sprintf("Invalid identifier type '%s'", idType)).Send()
		return
	}

	var user models.User
	result := dbQuery.First(&user)
	if result.Error == gorm.ErrRecordNotFound {
		gecho.NotFound(w).WithMessage(fmt.Sprintf("No user with %s of '%s'", idType, idStr)).Send()
		return
	}
	if result.Error != nil {
		gecho.InternalServerError(w).Send()
		logger.Err(result.Error.Error())
		return
	}

	userInfo := toUserInfo(user)

	gecho.Success(w).WithData(userInfo).Send()
}
