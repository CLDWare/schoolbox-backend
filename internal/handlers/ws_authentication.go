package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/big"
	"time"

	"github.com/CLDWare/schoolbox-backend/pkg/db"
	"github.com/CLDWare/schoolbox-backend/pkg/logger"
	"gorm.io/gorm"
)

func triggersAuthenticationFlow(message *websocketMessage) bool {
	for _, value := range [2]string{"auth_start", "auth_validate"} {
		if value == message.Command {
			return true
		}
	}
	return false
}

type authenticationFlowData struct {
	startedAt   time.Time
	flowTimeout uint
	targetID    uint
	nonce       string
}

type websocketAuthStartMessage struct {
	Command  string
	TargetID uint
}

func toWebsocketAuthStartMessage(m websocketMessage) (websocketAuthStartMessage, *websocketErrorMessage) {
	if m.Command != "auth_start" {
		errCode := -1
		errMsg := fmt.Sprintf("websocketMessage should have command 'auth_start', not '%s'", m.Command)
		return websocketAuthStartMessage{}, &websocketErrorMessage{ErrorCode: errCode, Info: &errMsg} // internal server error
	}
	id, ok := m.Data["id"]
	if !ok {
		errCode := 0
		errMsg := "No data field 'id'"
		return websocketAuthStartMessage{}, &websocketErrorMessage{ErrorCode: errCode, Info: &errMsg} // bad request
	}

	switch v := id.(type) {
	case float64:
		// JSON numbers are float64 by default
		if v < 0 || v != math.Trunc(v) {
			errCode := 0
			errMsg := "invalid id: must be a non-negative integer"
			return websocketAuthStartMessage{}, &websocketErrorMessage{ErrorCode: errCode, Info: &errMsg} // bad request
		}
		return websocketAuthStartMessage{Command: "auth_start", TargetID: uint(v)}, nil
	default:
		errCode := 0
		errMsg := fmt.Sprintf("invalid id: unsupported type %T", id)
		return websocketAuthStartMessage{}, &websocketErrorMessage{ErrorCode: errCode, Info: &errMsg} // bad request
	}
}

type websocketAuthValidateMessage struct {
	Command   string
	Signature string
}

func toWebsocketAuthValidateMessage(m websocketMessage) (websocketAuthValidateMessage, *websocketErrorMessage) {
	if m.Command != "auth_validate" {
		errCode := -1
		errMsg := fmt.Sprintf("websocketMessage should have command 'auth_validate', not '%s'", m.Command)
		return websocketAuthValidateMessage{}, &websocketErrorMessage{ErrorCode: errCode, Info: &errMsg} // internal server error
	}
	id, ok := m.Data["signature"]
	if !ok {
		errCode := 0
		errMsg := "No data field 'signature'"
		return websocketAuthValidateMessage{}, &websocketErrorMessage{ErrorCode: errCode, Info: &errMsg} // bad request
	}

	switch v := id.(type) {
	case string:
		return websocketAuthValidateMessage{Command: "auth_validate", Signature: v}, nil
	default:
		errCode := 0
		errMsg := fmt.Sprintf("invalid signature: unsupported type %T", id)
		return websocketAuthValidateMessage{}, &websocketErrorMessage{ErrorCode: errCode, Info: &errMsg} // bad request
	}
}

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func generateNonce() (string, error) {
	b := make([]byte, 128)
	for i := range b {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		b[i] = charset[num.Int64()]
	}
	return string(b), nil
}

func authenticationFlow(conn *websocketConnection, message websocketMessage) error {
	switch message.Command {
	case "auth_start":
		conn.mu.RLock()
		if conn.state != 0 {
			conn.mu.RUnlock()
			errCode := 0
			errMsg := fmt.Sprintf("Can not start authentication in current state %d, only state 0 is allowed", conn.state)
			sendMessage(conn.ws, websocketErrorMessage{ErrorCode: errCode, Info: &errMsg}) // invalid state
			return nil
		}
		conn.mu.RUnlock()

		message, parseErr := toWebsocketAuthStartMessage(message)
		if parseErr != nil {
			sendMessage(conn.ws, parseErr)
			return nil
		}
		ctx := context.Background()

		conn.mu.Lock()
		conn.state = 2
		conn.mu.Unlock()

		id := message.TargetID
		_, err := gorm.G[db.Device](conn.db).Where("id = ?", id).First(ctx)
		if errors.Is(err, gorm.ErrRecordNotFound) {

		}
		if err != nil {
			errCode := -1
			errMsg := err.Error()
			sendMessage(conn.ws, websocketErrorMessage{ErrorCode: errCode, Info: &errMsg}) // internal server error
			return nil
		}

		nonce, err := generateNonce()
		if err != nil {
			errCode := -1
			errMsg := err.Error()
			sendMessage(conn.ws, websocketErrorMessage{ErrorCode: errCode, Info: &errMsg}) // internal server error
			return nil
		}

		conn.mu.Lock()
		conn.stateFlow = authenticationFlowData{
			startedAt:   time.Now(),
			flowTimeout: 30,
			targetID:    id,
			nonce:       nonce,
		}
		conn.mu.Unlock()
		logger.Info(fmt.Sprintf("Started authentication for device %d", id))

		command := "auth_nonce"
		data := map[string]any{
			"nonce": nonce,
		}
		sendMessage(conn.ws, websocketMessage{Command: command, Data: data})
	case "auth_validate":
		conn.mu.RLock()
		if conn.state != 2 {
			conn.mu.RUnlock()
			errCode := 0
			errMsg := fmt.Sprintf("Can not validate authentication in current state %d, only state 2 is allowed", conn.state)
			sendMessage(conn.ws, websocketErrorMessage{ErrorCode: errCode, Info: &errMsg}) // invalid state
			return nil
		}
		conn.mu.RUnlock()

		message, parseErr := toWebsocketAuthValidateMessage(message)
		if parseErr != nil {
			sendMessage(conn.ws, parseErr)
			return nil
		}

		flowData, ok := conn.stateFlow.(authenticationFlowData)
		if !ok {
			errCode := -1
			errMsg := fmt.Sprintf("Fatal: Invalid stateFlow type of %T, not authenticationFlowData", conn.stateFlow)
			sendMessage(conn.ws, websocketErrorMessage{ErrorCode: errCode, Info: &errMsg}) // internal server error
			logger.Err(errMsg)
			conn.close()
			return errors.New(errMsg)
		}

		ctx := context.Background()

		device, err := gorm.G[db.Device](conn.db).Where("id = ?", flowData.targetID).First(ctx)
		if err != nil {
			errCode := -1
			errMsg := fmt.Sprintf("Could not retrieve device %d from database", flowData.targetID)
			sendMessage(conn.ws, websocketErrorMessage{ErrorCode: errCode, Info: &errMsg}) // internal server error
			conn.state = 0
			conn.stateFlow = nil
			return nil
		}

		decodedSignature, err := hex.DecodeString(message.Signature)
		if err != nil {
			errCode := 3
			errMsg := "Invalid signature encoding."
			sendMessage(conn.ws, websocketErrorMessage{ErrorCode: errCode, Info: &errMsg}) // invalid auth data
			conn.mu.Lock()
			conn.state = 0
			conn.stateFlow = nil
			conn.mu.Unlock()
			return nil
		}

		mac := hmac.New(sha256.New, []byte(device.Token))
		mac.Write([]byte(flowData.nonce))
		expectedMAC := mac.Sum(nil)
		if !hmac.Equal(decodedSignature, expectedMAC) {
			errCode := 3
			errMsg := "Invalid signature."
			sendMessage(conn.ws, websocketErrorMessage{ErrorCode: errCode, Info: &errMsg}) // invalid auth data
			conn.mu.Lock()
			conn.state = 0
			conn.stateFlow = nil
			conn.mu.Unlock()

			logger.Info(fmt.Sprintf(
				"Auth fail for device %d, Invalid signature. Got '%s', expected '%s'",
				flowData.targetID,
				message.Signature,
				hex.EncodeToString(expectedMAC),
			))

			return nil
		}

		conn.mu.Lock()
		conn.state = 3
		conn.stateFlow = nil
		conn.deviceID = &flowData.targetID

		// Kick old device
		if conn.handler.connectedDevices[*conn.deviceID] != 0 {
			oldConn := conn.handler.connections[conn.handler.connectedDevices[*conn.deviceID]]
			errCode := 4
			errMsg := "Logged in at other place. Only one connection allowed per device."
			sendMessage(oldConn.ws, websocketErrorMessage{ErrorCode: errCode, Info: &errMsg}) // multiple logins
			oldConn.close()
		}

		conn.handler.connectedDevices[*conn.deviceID] = conn.connectionID
		conn.handler.mu.Unlock()

		sendMessage(conn.ws, websocketMessage{Command: "auth_ok"})
		logger.Info(fmt.Sprintf("Device %d authenticated successfully", *conn.deviceID))
	default:
		logger.Err(fmt.Sprintf("Invalid command '%s' reached authenticationFlow", message.Command))
	}
	return nil
}
