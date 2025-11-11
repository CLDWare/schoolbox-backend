package handlers

import (
	"fmt"
	"math/rand"
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

func registrationFlow(conn *websocketConnection, message websocketMessage) *error {
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

		command := "reg_pin"
		data := map[string]interface{}{
			"pin": pin,
		}
		sendMessage(*conn.ws, websocketMessage{Command: command, Data: data})
	}
	return nil
}
