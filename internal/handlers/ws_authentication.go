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

func toWebsocketAuthStartMessage(m websocketMessage) (websocketAuthStartMessage, error) {
	if m.Command != "auth_start" {
		return websocketAuthStartMessage{}, fmt.Errorf("websocketMessage should have command 'auth_start', not '%s'", m.Command)
	}
	id, ok := m.Data["id"]
	if !ok {
		return websocketAuthStartMessage{}, errors.New("No data field 'id'")
	}

	switch v := id.(type) {
	case float64:
		// JSON numbers are float64 by default
		if v < 0 || v != math.Trunc(v) {
			return websocketAuthStartMessage{}, errors.New("invalid id: must be a non-negative integer")
		}
		return websocketAuthStartMessage{Command: "auth_start", TargetID: uint(v)}, nil
	default:
		return websocketAuthStartMessage{}, fmt.Errorf("invalid id: unsupported type %T", id)
	}
}

type websocketAuthValidateMessage struct {
	Command   string
	Signature string
}

func toWebsocketAuthValidateMessage(m websocketMessage) (websocketAuthValidateMessage, error) {
	if m.Command != "auth_validate" {
		return websocketAuthValidateMessage{}, fmt.Errorf("websocketMessage should have command 'auth_validate', not '%s'", m.Command)
	}
	id, ok := m.Data["signature"]
	if !ok {
		return websocketAuthValidateMessage{}, errors.New("No data field 'signature'")
	}

	switch v := id.(type) {
	case string:
		return websocketAuthValidateMessage{Command: "auth_validate", Signature: v}, nil
	default:
		return websocketAuthValidateMessage{}, fmt.Errorf("invalid signature: unsupported type %T", id)
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
		if conn.state != 0 {
			errCode := uint(0)
			errMsg := fmt.Sprintf("Can not start authentication in current state %d, only state 0 is allowed", conn.state)
			sendMessage(*conn.ws, websocketErrorMessage{ErrorCode: errCode, Info: &errMsg})
			return nil
		}

		message, err := toWebsocketAuthStartMessage(message)
		if err != nil {
			errCode := uint(0)
			errMsg := err.Error()
			sendMessage(*conn.ws, websocketErrorMessage{ErrorCode: errCode, Info: &errMsg})
			return nil
		}

		conn.state = 2

		id := message.TargetID
		nonce, err := generateNonce()
		if err != nil {

		}

		conn.stateFlow = authenticationFlowData{
			startedAt:   time.Now(),
			flowTimeout: 30,
			targetID:    id,
			nonce:       nonce,
		}
		logger.Info(fmt.Sprintf("Started authentication for device %d", id))

		command := "auth_nonce"
		data := map[string]any{
			"nonce": nonce,
		}
		sendMessage(*conn.ws, websocketMessage{Command: command, Data: data})
	case "auth_validate":
		if conn.state != 2 {
			errCode := uint(0)
			errMsg := fmt.Sprintf("Can not validate authentication in current state %d, only state 2 is allowed", conn.state)
			sendMessage(*conn.ws, websocketErrorMessage{ErrorCode: errCode, Info: &errMsg})
			return nil
		}
		message, err := toWebsocketAuthValidateMessage(message)
		if err != nil {
			errCode := uint(0)
			errMsg := err.Error()
			sendMessage(*conn.ws, websocketErrorMessage{ErrorCode: errCode, Info: &errMsg})
			return nil
		}

		flowData, ok := conn.stateFlow.(authenticationFlowData)
		if !ok {
			errCode := uint(0)
			errMsg := fmt.Sprintf("Fatal: Invalid stateFlow type of %T, not authenticationFlowData", conn.stateFlow)
			sendMessage(*conn.ws, websocketErrorMessage{ErrorCode: errCode, Info: &errMsg})
			logger.Err(errMsg)
			conn.ws.Close()
			return errors.New(errMsg)
		}

		ctx := context.Background()

		device, err := gorm.G[db.Device](conn.db).Where("id = ?", flowData.targetID).First(ctx)
		if err != nil {
			errCode := uint(0)
			errMsg := fmt.Sprintf("Could not retrieve device from database")
			sendMessage(*conn.ws, websocketErrorMessage{ErrorCode: errCode, Info: &errMsg})
			conn.state = 0
			conn.stateFlow = nil
			return nil
		}

		decodedSignature, err := hex.DecodeString(message.Signature)
		if err != nil {
			errCode := uint(3)
			errMsg := "Invalid signature encoding"
			sendMessage(*conn.ws, websocketErrorMessage{ErrorCode: errCode, Info: &errMsg})
			conn.state = 0
			conn.stateFlow = nil
			return nil
		}

		mac := hmac.New(sha256.New, []byte(device.Token))
		mac.Write([]byte(flowData.nonce))
		expectedMAC := mac.Sum(nil)
		if !hmac.Equal(decodedSignature, expectedMAC) {
			errCode := uint(3)
			errMsg := "Invalid signature."
			sendMessage(*conn.ws, websocketErrorMessage{ErrorCode: errCode, Info: &errMsg})
			conn.state = 0
			conn.stateFlow = nil

			logger.Info(fmt.Sprintf(
				"Auth fail for device %d, Invalid signature. Got '%s', expected '%s'",
				flowData.targetID,
				message.Signature,
				hex.EncodeToString(expectedMAC),
			))

			return nil
		}

		conn.state = 3
		conn.stateFlow = nil
		conn.deviceID = &flowData.targetID

		// Kick old device
		if conn.handler.connectedDevices[*conn.deviceID] != 0 {
			oldConn := conn.handler.connections[conn.handler.connectedDevices[*conn.deviceID]]
			errCode := uint(4)
			errMsg := "Logged in at other place. Only one connection allowed per device."
			sendMessage(*oldConn.ws, websocketErrorMessage{ErrorCode: errCode, Info: &errMsg})
			oldConn.close()
		}

		conn.handler.connectedDevices[*conn.deviceID] = conn.connectionID

		sendMessage(*conn.ws, websocketMessage{Command: "auth_ok"})
		logger.Info(fmt.Sprintf("Device %d authenticated successfully", *conn.deviceID))
	default:
		logger.Err(fmt.Sprintf("Invalid command '%s' reached authenticationFlow", message.Command))
	}
	return nil
}
