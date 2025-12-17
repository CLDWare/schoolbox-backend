package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/CLDWare/schoolbox-backend/config"
	contextkeys "github.com/CLDWare/schoolbox-backend/internal/contextKeys"
	models "github.com/CLDWare/schoolbox-backend/pkg/db"
	"github.com/CLDWare/schoolbox-backend/pkg/logger"
	"github.com/MonkyMars/gecho"
	"gorm.io/gorm"
)

// SessionHandler handles requests about sessions
type SessionHandler struct {
	config           *config.Config
	db               *gorm.DB
	sessionMan       *SessionManager
	websocketHandler *WebsocketHandler
}

// NewSessionHandler creates a new SessionHandler
func NewSessionHandler(cfg *config.Config, db *gorm.DB, websocketHandler *WebsocketHandler) *SessionHandler {
	return &SessionHandler{
		config:           cfg,
		db:               db,
		sessionMan:       NewSessionManager(),
		websocketHandler: websocketHandler,
	}
}

type SessionManager struct {
	sessionsByUser   map[uint]*uint
	sessionsByDevice map[uint]*uint
	mu               sync.RWMutex
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessionsByUser:   make(map[uint]*uint),
		sessionsByDevice: make(map[uint]*uint),
	}
}

func (sm *SessionManager) addSession(session *models.Session) {
	sm.mu.Lock()
	sm.sessionsByUser[session.UserID] = &session.ID
	sm.sessionsByDevice[session.DeviceID] = &session.ID
	sm.mu.Unlock()
}
func (sm *SessionManager) removeSession(session *models.Session) {
	sm.mu.Lock()
	delete(sm.sessionsByUser, session.UserID)
	delete(sm.sessionsByDevice, session.DeviceID)
	sm.mu.Unlock()
}

func toSessionInfo(session models.Session) map[string]any {
	return map[string]any{
		"id":              session.ID,
		"userID":          session.UserID,
		"questionID":      session.QuestionID,
		"question":        session.Question.Question,
		"deviceID":        session.DeviceID,
		"date":            session.Date,
		"stopped_at":      session.StoppedAt,
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
	err := dbQuery.Order("date DESC").Preload("Question").Find(&sessions).Error // retrieve sessions, sorted by date (newest first)
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

type PostSessionBody struct {
	DeviceID *uint   `json:"device_id"`
	Question *string `json:"question"`
}

// handles POST /session requests
// Any user can POST this endpoint to start a session (if they dont have an active one)
func (h *SessionHandler) PostSession(w http.ResponseWriter, r *http.Request) {
	if err := gecho.Handlers.HandleMethod(w, r, http.MethodPost); err != nil {
		err.Send() // Automatically sends 405 Method Not Allowed
		return
	}
	ctx := r.Context()
	user, ok := ctx.Value(contextkeys.AuthUserKey).(models.User)
	if !ok {
		gecho.InternalServerError(w).Send()
	}

	h.sessionMan.mu.RLock()
	if h.sessionMan.sessionsByUser[user.ID] != nil {
		h.sessionMan.mu.RUnlock()
		gecho.NewErr(w).WithStatus(http.StatusConflict).WithMessage("Can not have more than 1 session").Send()
		return
	}
	h.sessionMan.mu.RUnlock()

	var body PostSessionBody
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		errMsg := fmt.Sprintf("Error while decoding json: %E", err)
		logger.Err(errMsg)
		gecho.BadRequest(w).WithMessage(errMsg).Send()
		return
	}
	if body.DeviceID == nil {
		gecho.BadRequest(w).WithMessage("Missing field 'device_id'").Send()
		return
	}
	if body.Question == nil {
		gecho.BadRequest(w).WithMessage("Missing field 'question'").Send()
		return
	}

	session, err := h.websocketHandler.startSession(user.ID, *body.DeviceID, *body.Question)
	if err == ErrDeviceNotConnected {
		gecho.ServiceUnavailable(w).WithMessage("Device currently unavailable").Send()
		return
	} else if err != nil {
		gecho.InternalServerError(w).Send()
		logger.Err(err)
		return
	}

	h.sessionMan.addSession(session)

	sessionInfo := toSessionInfo(*session)

	gecho.Success(w).WithData(sessionInfo).Send()
}

func (h *SessionHandler) StopSession(w http.ResponseWriter, ctx context.Context, sessionID uint) *models.Session {
	h.db.Model(&models.Session{}).
		Where("id = ?", sessionID).
		UpdateColumn("stopped_at", time.Now())

	session, err := gorm.G[models.Session](h.db).Preload("Question", nil).Where("id = ?", sessionID).First(ctx)
	if err == gorm.ErrRecordNotFound {
		gecho.InternalServerError(w).WithMessage(fmt.Sprintf("No session with id: %d", sessionID)).Send()
		return nil
	}
	if err != nil {
		logger.Err(err.Error())
		gecho.InternalServerError(w).Send()
		return nil
	}

	_, err = gorm.G[models.Device](h.db).Where("id = ?", session.DeviceID).Update(ctx, "active_session_id", nil)
	if err != nil {
		logger.Err(err.Error())
	}

	h.sessionMan.removeSession(&session)
	h.websocketHandler.stopSession(&session)

	return &session
}

// handles POST /session/stop requests
// Any user can POST this endpoint to stop their own session
func (h *SessionHandler) PostSessionStop(w http.ResponseWriter, r *http.Request) {
	if err := gecho.Handlers.HandleMethod(w, r, http.MethodPost); err != nil {
		err.Send() // Automatically sends 405 Method Not Allowed
		return
	}

	ctx := r.Context()
	user, ok := ctx.Value(contextkeys.AuthUserKey).(models.User)
	if !ok {
		gecho.InternalServerError(w).Send()
	}

	h.sessionMan.mu.RLock()
	sessionID := h.sessionMan.sessionsByUser[user.ID]
	h.sessionMan.mu.RUnlock()
	if sessionID == nil {
		gecho.NotFound(w).WithMessage("No current session").Send()
		return
	}

	session := h.StopSession(w, ctx, *sessionID)

	sessionInfo := toSessionInfo(*session)

	gecho.Success(w).WithData(sessionInfo).Send()
}

// handles POST /session/{id}/stop requests
// Admins can POST this endpoint to stop any session
func (h *SessionHandler) PostSessionStopById(w http.ResponseWriter, r *http.Request) {
	if err := gecho.Handlers.HandleMethod(w, r, http.MethodPost); err != nil {
		err.Send() // Automatically sends 405 Method Not Allowed
		return
	}

	ctx := r.Context()

	sessionIDStr := r.PathValue("id")
	sessionID, err := strconv.ParseUint(sessionIDStr, 10, 0)
	if err != nil {
		gecho.BadRequest(w).WithMessage("Invalid session ID, expected positive integer").Send()
		return
	}

	session := h.StopSession(w, ctx, uint(sessionID))

	sessionInfo := toSessionInfo(*session)

	gecho.Success(w).WithData(sessionInfo).Send()
}

// handles GET /session/current requests
// Any user can query this endpoint for their own session
func (h *SessionHandler) GetCurrentSession(w http.ResponseWriter, r *http.Request) {
	if err := gecho.Handlers.HandleMethod(w, r, http.MethodGet); err != nil {
		err.Send() // Automatically sends 405 Method Not Allowed
		return
	}

	ctx := r.Context()
	user, ok := ctx.Value(contextkeys.AuthUserKey).(models.User)
	if !ok {
		gecho.InternalServerError(w).Send()
	}

	h.sessionMan.mu.RLock()
	sessionID := h.sessionMan.sessionsByUser[user.ID]
	h.sessionMan.mu.RUnlock()
	if sessionID == nil {
		gecho.NotFound(w).WithMessage("No current session").Send()
		return
	}

	session, err := gorm.G[models.Session](h.db).Preload("Question", nil).Where("id = ?", sessionID).First(ctx)
	if err == gorm.ErrRecordNotFound {
		gecho.InternalServerError(w).WithMessage(fmt.Sprintf("No session with id: %d", sessionID)).Send()
		return
	}
	if err != nil {
		logger.Err(err.Error())
		gecho.InternalServerError(w).Send()
		return
	}

	sessionInfo := toSessionInfo(session)

	gecho.Success(w).WithData(sessionInfo).Send()
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

	session, err := gorm.G[models.Session](h.db).Preload("Question", nil).Where("id = ?", sessionID).First(ctx)
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
