package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/CLDWare/schoolbox-backend/config"
	contextkeys "github.com/CLDWare/schoolbox-backend/internal/contextKeys"
	models "github.com/CLDWare/schoolbox-backend/pkg/db"
	"github.com/CLDWare/schoolbox-backend/pkg/logger"
	"github.com/MonkyMars/gecho"
	"gorm.io/gorm"
)

// SessionHandler handles requests about sessions
type SessionHandler struct {
	config *config.Config
	db     *gorm.DB
}

// NewSessionHandler creates a new SessionHandler
func NewSessionHandler(cfg *config.Config, db *gorm.DB) *SessionHandler {
	return &SessionHandler{
		config: cfg,
		db:     db,
	}
}

func toSessionInfo(session models.Session) map[string]any {
	return map[string]any{
		"id":              session.ID,
		"userID":          session.UserID,
		"questionID":      session.QuestionID,
		"question":        session.Question.Question,
		"deviceID":        session.DeviceID,
		"date":            session.Date,
		"firstAnwserTime": session.FirstAnwserTime,
		"lastAnwserTime":  session.LastAnwserTime,
		"votes": [5]uint16{
			session.A1_count,
			session.A2_count,
			session.A3_count,
			session.A4_count,
			session.A5_count,
		},
	}
}

// handles GET /session requests
// Any user can query this endpoint for their own sessions
// Privileged users can add asRole=1 query parameter to act with their privileges
func (h *SessionHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	if err := gecho.Handlers.HandleMethod(w, r, http.MethodGet); err != nil {
		err.Send() // Automatically sends 405 Method Not Allowed
		return
	}
	ctx := r.Context()
	user, ok := ctx.Value(contextkeys.AuthUserKey).(models.User)
	if !ok {
		gecho.InternalServerError(w).Send()
	}

	query := r.URL.Query()
	dbQuery := h.db.Model(&models.Session{})

	asRole := uint(0)
	if asRoleStr := query.Get("asRole"); asRoleStr != "" {
		asRoleParsed, err := strconv.ParseUint(asRoleStr, 10, 0)
		if err != nil {
			gecho.BadRequest(w).WithMessage(err.Error()).Send()
			return
		}
		if asRoleParsed != 0 && user.Role != uint(asRoleParsed) {
			gecho.Forbidden(w).Send()
			return
		}
		asRole = uint(asRoleParsed)
	}
	switch asRole {
	case 1:
		// privileged filters
		if userIDStr := query.Get("user_id"); userIDStr != "" {
			userID, err := strconv.Atoi(userIDStr)
			if err != nil {
				gecho.BadRequest(w).WithMessage(err.Error()).Send()
				return
			}
			dbQuery = dbQuery.Where("user_id = ?", userID)
		}
	case 0:
		dbQuery = dbQuery.Where("user_id = ?", user.ID)
	}

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
	if questionIDStr := query.Get("questionID"); questionIDStr != "" {
		questionID, err := strconv.Atoi(questionIDStr)
		if err != nil {
			gecho.BadRequest(w).WithMessage(err.Error()).Send()
			return
		}
		dbQuery = dbQuery.Where("questionID = ?", questionID)
	}

	var sessions []models.Session
	err := dbQuery.Order("date DESC").Find(&sessions).Error // retrieve sessions, sorted by date (newest first)
	if err != nil {
		logger.Err(err.Error())
		gecho.InternalServerError(w).Send()
		return
	}

	sessionInfoArray := []map[string]any{}
	for _, session := range sessions {
		sessionInfoArray = append(sessionInfoArray, toSessionInfo(session))
	}

	gecho.Success(w).WithData(sessionInfoArray).Send()
}

// handles GET /session/{id} requests
// Any user can query this endpoint for their own sessions
// Privileged users can add asRole=1 query parameter to act with their privileges
func (h *SessionHandler) GetSessionById(w http.ResponseWriter, r *http.Request) {
	if err := gecho.Handlers.HandleMethod(w, r, http.MethodGet); err != nil {
		err.Send() // Automatically sends 405 Method Not Allowed
		return
	}

	ctx := r.Context()
	user, ok := ctx.Value(contextkeys.AuthUserKey).(models.User)
	if !ok {
		gecho.InternalServerError(w).Send()
	}

	query := r.URL.Query()

	asRole := uint(0)
	if asRoleStr := query.Get("asRole"); asRoleStr != "" {
		asRoleParsed, err := strconv.ParseUint(asRoleStr, 10, 0)
		if err != nil {
			gecho.BadRequest(w).WithMessage(err.Error()).Send()
			return
		}
		if asRoleParsed != 0 && user.Role != uint(asRoleParsed) {
			gecho.Forbidden(w).Send()
			return
		}
		asRole = uint(asRoleParsed)
	}

	sessionIDStr := r.PathValue("id")
	sessionID, err := strconv.ParseUint(sessionIDStr, 10, 0)
	if err != nil {
		gecho.BadRequest(w).WithMessage("Invalid session ID, expected positive integer").Send()
		return
	}

	session, err := gorm.G[models.Session](h.db).Where("id = ?", sessionID).First(ctx) // retrieve sessions, sorted by date (newest first)
	if err == gorm.ErrRecordNotFound {
		gecho.NotFound(w).WithMessage(fmt.Sprintf("No session with id: %d", sessionID)).Send()
		return
	}
	if err != nil {
		logger.Err(err.Error())
		gecho.InternalServerError(w).Send()
		return
	}

	if asRole != 1 && user.ID != session.UserID {
		gecho.Forbidden(w).Send()
		return
	}

	sessionInfo := toSessionInfo(session)

	gecho.Success(w).WithData(sessionInfo).Send()
}
