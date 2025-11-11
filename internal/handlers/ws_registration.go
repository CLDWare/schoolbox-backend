package handlers

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
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
			sendMessage(*conn.ws, websocketErrorMessage{ErrorCode: errCode, Info: &errMsg})
			return nil
		}
		conn.state = 1
		pin := rand.Intn(9000) + 1000
		conn.stateFlow = registrationFlowData{pin: uint(pin)}

		conn.handler.registrationPins[pin] = conn.connectionID

		command := "reg_pin"
		data := map[string]interface{}{
			"pin": pin,
		}
		sendMessage(*conn.ws, websocketMessage{Command: command, Data: data})
	}
	return nil
}

func (h *WebsocketHandler) registerWithPin(pin int) error {
	connectionID := h.registrationPins[pin]
	conn := h.connections[connectionID]

	token, err := generateSecureToken(128)
	if err != nil {
		logger.Err(fmt.Sprintf("An Error occured while generating secure token for %d: %s", conn.connectionID, err))
		return err
	}

	ctx := context.Background()

	device := models.Device{
		Token: token,
	}
	err = gorm.G[models.Device](h.db).Create(ctx, &device)

	command := "reg_ok"
	data := map[string]any{
		"id":    device.ID,
		"token": token,
	}
	sendMessage(*conn.ws, websocketMessage{Command: command, Data: data})

	conn.state = 0
	conn.stateFlow = nil

	logger.Info(fmt.Sprintf("Registered new device with ID %d", device.ID))

	return nil
}
