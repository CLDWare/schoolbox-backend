package handlers

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	models "github.com/CLDWare/schoolbox-backend/pkg/db"
	"github.com/CLDWare/schoolbox-backend/pkg/logger"
	"gorm.io/gorm"
)

func triggersSessionFlow(message *websocketMessage) bool {
	for _, value := range [2]string{"session_vote"} {
		if value == message.Command {
			return true
		}
	}
	return false
}

type sessionFlowData struct {
	sessionID uint
	started   time.Time
}

type sessionVoteMessage struct {
	Command string
	Vote    uint
}

func toSessionVoteMessage(m websocketMessage) (sessionVoteMessage, *websocketErrorMessage) {
	if m.Command != "session_vote" {
		errCode := -1
		errMsg := fmt.Sprintf("sessionVoteMessage should have command 'session_vote', not '%s'", m.Command)
		return sessionVoteMessage{}, &websocketErrorMessage{ErrorCode: errCode, Info: &errMsg} // internal server error
	}
	vote, ok := m.Data["vote"]
	if !ok {
		errCode := 0
		errMsg := "No data field 'vote'"
		return sessionVoteMessage{}, &websocketErrorMessage{ErrorCode: errCode, Info: &errMsg} // bad request
	}

	switch v := vote.(type) {
	case float64:
		// JSON numbers are float64 by default
		if v < 1 || v > 5 || v != math.Trunc(v) {
			errCode := 0
			errMsg := "Invalid vote: must be a non-negative integer between 1 and 5 (inclusive)"
			return sessionVoteMessage{}, &websocketErrorMessage{ErrorCode: errCode, Info: &errMsg} // bad request
		}

		return sessionVoteMessage{Command: "session_vote", Vote: uint(v)}, nil
	default:
		errCode := 0
		errMsg := fmt.Sprintf("Invalid vote: unsupported type %T", vote)
		return sessionVoteMessage{}, &websocketErrorMessage{ErrorCode: errCode, Info: &errMsg} // bad request
	}
}

func sessionFlow(conn *websocketConnection, message websocketMessage) error {
	switch message.Command {
	case "session_vote":
		if conn.state != 4 {
			errCode := 0
			errMsg := fmt.Sprintf("Can not vote while not in session. current state %d, only state 4 is allowed", conn.state)
			sendMessage(conn.ws, websocketErrorMessage{ErrorCode: errCode, Info: &errMsg}) // invalid state
			return nil
		}

		message, parseErr := toSessionVoteMessage(message)
		if parseErr != nil {
			sendMessage(conn.ws, parseErr)
			return nil
		}

		flowData, ok := conn.stateFlow.(sessionFlowData)
		if !ok {
			errCode := -1
			errMsg := fmt.Sprintf("Fatal: Invalid stateFlow type of %T, not sessionFlowData", conn.stateFlow)
			sendMessage(conn.ws, websocketErrorMessage{ErrorCode: errCode, Info: &errMsg}) // internal server error
			logger.Err(errMsg)
			conn.close()
			return errors.New(errMsg)
		}
		column := fmt.Sprintf("A%d_count", message.Vote)
		expr := gorm.Expr(fmt.Sprintf("%s + 1", column))
		conn.handler.db.Model(&models.Session{}).Where("id = ?", flowData.sessionID).UpdateColumn(column, expr)
	default:
		err := fmt.Errorf("Invalid command '%s' reached sessionFLow", message.Command)
		logger.Err(err)
		return err
	}
	return nil
}

func (h *WebsocketHandler) startSession(userID uint, deviceID uint, questionStr string) (map[string]any, error) {
	ctx := context.Background()

	question := models.Question{
		Question: questionStr,
	}
	result := h.db.FirstOrCreate(ctx, &question)
	if result.Error != nil {
		err := fmt.Errorf("An error occured retrieving/creating the question: %s", result.Error)
		logger.Err(err)
		return nil, err
	}

	connID, ok := h.connectedDevices[deviceID]
	if !ok {
		err := fmt.Errorf("No device %d is currently connected, can not start session", deviceID)
		logger.Err(err)
		return nil, err
	}
	conn, ok := h.connections[connID]
	if !ok {
		err := fmt.Errorf("Connection %d for device %d does not exist", connID, deviceID)
		logger.Err(err)
		delete(h.connectedDevices, deviceID) // remove device from connectedDevices map because the connection no longer exists
		return nil, err
	}

	session := models.Session{
		UserID:     userID,
		QuestionID: question.ID,
		DeviceID:   deviceID,
		Date:       time.Now(),
	}
	err := gorm.G[models.Session](h.db).Create(ctx, &session)
	if err != nil {
		logger.Err(err)
		return nil, err
	}
	flowData := sessionFlowData{
		sessionID: session.ID,
	}
	conn.state = 4
	conn.stateFlow = flowData

	command := "session_start"
	data := map[string]any{
		"text": question.Question,
	}
	sendMessage(conn.ws, websocketMessage{
		Command: command,
		Data:    data,
	})

	return nil, nil
}
