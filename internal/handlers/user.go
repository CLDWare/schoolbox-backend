package handlers

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/CLDWare/schoolbox-backend/config"
	contextkeys "github.com/CLDWare/schoolbox-backend/internal/contextKeys"
	models "github.com/CLDWare/schoolbox-backend/pkg/db"
	"github.com/CLDWare/schoolbox-backend/pkg/logger"
	"github.com/MonkyMars/gecho"
	"gorm.io/gorm"
)

// UserHandler handles requests about users
type UserHandler struct {
	quitCh chan os.Signal
	config *config.Config
	db     *gorm.DB
}

// NewUserHandler creates a new UserHandler
func NewUserHandler(quitCh chan os.Signal, cfg *config.Config, db *gorm.DB) *UserHandler {
	return &UserHandler{
		quitCh: quitCh,
		config: cfg,
		db:     db,
	}
}

type UserInfo struct {
	ID              uint      `json:"id"`
	Email           string    `json:"email" format:"email"`
	GoogleSubject   string    `json:"google_sub" example:"012345678901234567890"`
	ProfilePicture  string    `json:"picture_url" format:"url"`
	Role            uint      `json:"role"`
	CreatedAt       time.Time `json:"joinedAt" format:"date-time"`
	Name            string    `json:"name" format:"name"`
	DisplayName     string    `json:"display_name"`
	DefaultQuestion string    `json:"default_question"`
}

func toUserInfo(user models.User) UserInfo {
	return UserInfo{
		ID:              user.ID,
		Email:           user.Email,
		GoogleSubject:   user.GoogleSubject,
		ProfilePicture:  user.ProfilePicture,
		Role:            user.Role,
		CreatedAt:       user.CreatedAt,
		Name:            user.Name,
		DisplayName:     user.DisplayName,
		DefaultQuestion: user.DefaultQuestion,
	}
}

// GetMe
//
// @Summary		Get UserInfo about current authenticated user
// @Description
// @Tags			user requiresAuth
// @Accept			json
// @Produce		json
// @Success		200	{object}	apiResponses.BaseResponse{data=UserInfo}
// @Failure		401	{object}	apiResponses.UnauthorizedError
// @Failure		500	{object}	apiResponses.InternalServerError
// @Router			/me [get]
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

// GetUser
//
// @Summary		Get all users
// @Description	Get UserInfo about all users
// @Tags			user requiresAuth requiresAdmin
// @Accept			json
// @Produce		json
// @Param			limit	query		int	false	"Amount of users to return" default(20) maximum(20)
// @Param			offset	query		int	false	"How much users to skip before starting to return users" default(0) minimum(0)
// @Param			role	query		int	false	"Only return users with this role" Enums(0,1)
// @Success		200	{object}	apiResponses.BaseResponse{data=[]UserInfo}
// @Failure		401	{object}	apiResponses.UnauthorizedError
// @Failure		403	{object}	apiResponses.ForbiddenError
// @Failure		500	{object}	apiResponses.InternalServerError
// @Router			/user [get]
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

	userInfoArray := []UserInfo{}
	for _, user := range users {
		userInfoArray = append(userInfoArray, toUserInfo(user))
	}

	gecho.Success(w).WithData(userInfoArray).Send()
}

// GetUserById
//
// @Summary		Get user by id
// @Description	Get info about a user by using either their id or email
// @Tags			user requiresAuth requiresAdmin
// @Accept			json
// @Produce		json
// @Param			id	path		string	true	"User ID or email"
// @Param			type	query		string	false	"Specify identifier type" Enums("id","email") default("id")
// @Success		200 {object}	apiResponses.BaseResponse{data=UserInfo}
// @Failure		401	{object}	apiResponses.UnauthorizedError
// @Failure		403	{object}	apiResponses.ForbiddenError
// @Failure		404	{object}	apiResponses.NotFoundError
// @Failure		500	{object}	apiResponses.InternalServerError
// @Router			/user/{id} [get]
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
