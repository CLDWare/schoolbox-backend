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
		if conn.state != 0 {
			errCode := uint(0)
			errMsg := fmt.Sprintf("Can not start registration in current state %d, only state 0 is allowed", conn.state)
			sendMessage(conn.ws, websocketErrorMessage{ErrorCode: errCode, Info: &errMsg})
			return nil
		}
		conn.state = 1
		pin := uint(rand.Intn(9000) + 1000)
		conn.stateFlow = registrationFlowData{pin: pin}

		conn.handler.registrationPins[pin] = conn.connectionID

		command := "reg_pin"
		data := map[string]any{
			"pin": pin,
		}
		sendMessage(conn.ws, websocketMessage{Command: command, Data: data})
		logger.Info(fmt.Sprintf("Started registration for connection %d with pin %d", conn.handler.registrationPins[pin], pin))
	}
	return nil
}

func (h *WebsocketHandler) registerWithPin(pin uint) (*models.Device, error) {
	connectionID, ok := h.registrationPins[pin]
	if !ok {
		logger.Info("Wrong pin provided for registration")
		return nil, errors.New("No connectionID for this pin")
	}
	conn, ok := h.connections[connectionID]
	if !ok {
		logger.Err(fmt.Sprintf("No connection for connectionID %d during registration with pin", connectionID))
		return nil, errors.New("No connection for connectionID")
	}

	token, err := generateSecureToken(128)
	if err != nil {
		logger.Err(fmt.Sprintf("An Error occured while generating secure token for %d: %s", conn.connectionID, err))
		return nil, err
	}

	ctx := context.Background()

	device := models.Device{
		Token: token,
	}
	err = gorm.G[models.Device](h.db).Create(ctx, &device)
	if err != nil {
		logger.Err(fmt.Sprintf("Error while creating device in database: %s", err.Error()))
		return nil, errors.New(err.Error())
	}

	command := "reg_ok"
	data := map[string]any{
		"id":    device.ID,
		"token": token,
	}
	sendMessage(conn.ws, websocketMessage{Command: command, Data: data})

	conn.state = 0
	conn.stateFlow = nil
	delete(h.registrationPins, pin)

	logger.Info(fmt.Sprintf("Registered new device with ID %d", device.ID))

	return &device, nil
}
