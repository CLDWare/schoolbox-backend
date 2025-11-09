package handlers

import (
	"fmt"
	"net/http"

	"github.com/CLDWare/schoolbox-backend/config"
	"github.com/CLDWare/schoolbox-backend/pkg/logger"

	"github.com/gorilla/websocket"
)

type WebsocketHandler struct {
	config *config.Config
}

func NewWebsocketHandler(cfg *config.Config) *WebsocketHandler {
	return &WebsocketHandler{
		config: cfg,
	}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // In production, check the origin properly!
	},
}

func (h *WebsocketHandler) InitialiseWebsocket(w http.ResponseWriter, r *http.Request) {
	logger.Init()

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Err(err)
		return
	}
	defer ws.Close()

	for {
		// Read message from client
		_, msg, err := ws.ReadMessage()
		if err != nil {
			logger.Err("read:", err)
			break
		}
		logger.Info(fmt.Sprintf("Received: %s", msg))

		// Write message back to client
		err = ws.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			logger.Err("write:", err)
			break
		}
	}
}
