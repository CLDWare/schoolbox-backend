package handlers

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"

	models "github.com/CLDWare/schoolbox-backend/pkg/db"
	"github.com/CLDWare/schoolbox-backend/pkg/logger"

	"gorm.io/gorm"
)

func triggersRegistrationFlow(message *websocketMessage) bool {
	for _, value := range [1]string{"reg_start"} {
		if value == message.Command {
			return true
		}
	}
	return false
}

type registrationFlowData struct {
	pin uint
}

func generateSecureToken(n int) (string, error) {
	// n is the number of bytes, not characters
	b := make([]byte, n)
	_, err := crand.Read(b)
	if err != nil {
		return "", err
	}

	// Encode as hexadecimal string
	return hex.EncodeToString(b), nil
}

func registrationFlow(conn *websocketConnection, message websocketMessage) error {
	if message.Command == "reg_start" {
		conn.mu.RLock()
		if conn.state != 0 {
			conn.mu.RUnlock()
			errCode := 0
			errMsg := fmt.Sprintf("Can not start registration in current state %d, only state 0 is allowed", conn.state)
			sendMessage(conn.ws, websocketErrorMessage{ErrorCode: errCode, Info: &errMsg}) // invalid state
			return nil
		}
		conn.mu.RUnlock()

		pin := uint(rand.Intn(9000) + 1000)

		conn.mu.Lock()
		conn.state = 1
		conn.stateFlow = registrationFlowData{pin: pin}
		conn.mu.Unlock()

		conn.handler.mu.Lock()
		conn.handler.registrationPins[pin] = conn.connectionID
		conn.handler.mu.Unlock()

		command := "reg_pin"
		data := map[string]any{
			"pin": pin,
		}
		sendMessage(conn.ws, websocketMessage{Command: command, Data: data})
		logger.Info(fmt.Sprintf("Started registration for connection %d with pin %d", conn.handler.registrationPins[pin], pin))
	}
	return nil
}

func (h *WebsocketHandler) registerWithPin(pin uint, device *models.Device) (*models.Device, error) {
	h.mu.RLock()
	connectionID, ok := h.registrationPins[pin]
	if !ok {
		h.mu.RUnlock()
		logger.Info("Wrong pin provided for registration")
		return nil, errors.New("No connectionID for this pin")
	}
	conn, ok := h.connections[connectionID]
	h.mu.RUnlock()
	if !ok {
		logger.Err(fmt.Sprintf("No connection for connectionID %d during registration with pin", connectionID))
		return nil, errors.New("No connection for connectionID")
	}
	h.mu.Lock() // Keep a lock on the handler so registerWithPin can not be called again until this registeration is successfull (prevent double registration)
	defer h.mu.Unlock()

	token, err := generateSecureToken(128)
	if err != nil {
		logger.Err(fmt.Sprintf("An Error occured while generating secure token for %d: %s", conn.connectionID, err))
		return nil, err
	}

	ctx := context.Background()

	if device == nil {
		device = &models.Device{
			Token: token,
		}

		err = gorm.G[models.Device](h.db).Create(ctx, device)
		if err != nil {
			logger.Err(fmt.Sprintf("Error while creating device in database during registration: %s", err.Error()))
			return nil, errors.New(err.Error())
		}
	} else {
		device.Token = token

		_, err = gorm.G[models.Device](h.db).Updates(ctx, *device)
		if err != nil {
			logger.Err(fmt.Sprintf("Error while updating device in database during relink: %s", err.Error()))
			return nil, errors.New(err.Error())
		}
	}

	command := "reg_ok"
	data := map[string]any{
		"id":    device.ID,
		"token": token,
	}
	sendMessage(conn.ws, websocketMessage{Command: command, Data: data})

	conn.mu.Lock()
	conn.state = 0
	conn.stateFlow = nil
	conn.mu.Unlock()

	delete(h.registrationPins, pin)

	logger.Info(fmt.Sprintf("Registered new device with ID %d", device.ID))

	return device, nil
}
