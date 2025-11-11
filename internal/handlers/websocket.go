package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/CLDWare/schoolbox-backend/config"
	"github.com/CLDWare/schoolbox-backend/pkg/logger"
	"gorm.io/gorm"

	"github.com/gorilla/websocket"
)

type WebsocketHandler struct {
	config           *config.Config
	db               *gorm.DB
	connections      map[uint]*websocketConnection
	nextID           uint
	connectedDevices map[uint]uint
	registrationPins map[uint]uint
}

func (h *WebsocketHandler) addConnection(conn *websocketConnection) {
	h.nextID = h.nextID + 1
	conn.handler = h
	conn.connectionID = h.nextID
	h.connections[h.nextID] = conn

	conn.ws.SetCloseHandler(func(code int, text string) error {
		return conn.close()
	})
}

type websocketConnection struct {
	handler         *WebsocketHandler
	connectionID    uint
	ws              *websocket.Conn
	db              *gorm.DB
	deviceID        *uint
	state           uint // 0 none;1 registering;2 authenticating;3 authenticated;
	stateFlow       any
	connectedAt     time.Time
	latestMessage   time.Time
	hearbeat_cancel context.CancelFunc
	latestHeartbeat time.Time
	pingsSent       uint
	pongsRecieved   uint
}

func (conn websocketConnection) close() error {
	conn.ws.Close()
	conn.stopHeartbeatMonitor()

	delete(conn.handler.connections, conn.connectionID)
	if conn.deviceID != nil {
		delete(conn.handler.connectedDevices, *conn.deviceID)
		logger.Info(fmt.Sprintf("Closed connection %d, device %d", conn.connectionID, *conn.deviceID))
	} else {
		logger.Info(fmt.Sprintf("Closed connection %d", conn.connectionID))
	}
	return nil
}

type websocketMessage struct {
	Command string         `json:"c,omitempty"`
	Data    map[string]any `json:"d,omitempty"`
}

type websocketErrorMessage struct {
	ErrorCode uint    `json:"e,omitempty"`
	Info      *string `json:"info,omitempty"`
}

func NewWebsocketHandler(cfg *config.Config, db *gorm.DB) *WebsocketHandler {
	return &WebsocketHandler{
		config:           cfg,
		db:               db,
		connections:      map[uint]*websocketConnection{},
		connectedDevices: map[uint]uint{},
		nextID:           0,
		registrationPins: map[uint]uint{},
	}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // In production, check the origin properly!
	},
}

func sendMessage(ws websocket.Conn, msg any) error {
	message, err := json.Marshal(msg)
	if err != nil {
		logger.Err("JSON marshal err: ", err)
		return err
	}
	err = ws.WriteMessage(websocket.TextMessage, message)
	if err != nil {
		logger.Err("write:", err)
	}
	return err
}

func (h *WebsocketHandler) InitialiseWebsocket(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Err(err)
		return
	}
	defer ws.Close()

	conn := websocketConnection{
		handler:       h,
		ws:            ws,
		db:            h.db,
		connectedAt:   time.Now(),
		latestMessage: time.Now(),
	}
	h.addConnection(&conn)
	conn.startHeartbeatMonitor()
	logger.Info(fmt.Sprintf("New connection %d", conn.connectionID))

	for {
		// Read message from client
		_, msg, err := ws.ReadMessage()
		if err != nil {
			logger.Err("read:", err)
			break
		}
		conn.latestMessage = time.Now()

		var message websocketMessage
		err = json.Unmarshal(msg, &message)
		if err != nil {
			logger.Err("Invalid JSON:", err)
			errCode := uint(0)
			errMsg := err.Error()
			sendErr := sendMessage(*conn.ws, websocketErrorMessage{ErrorCode: errCode, Info: &errMsg})
			if sendErr != nil {
				break
			}
			continue
		}

		logger.Info(fmt.Sprintf("Received: %s", msg))

		if message.Command == "" {
			errCode := uint(0)
			errMsg := "A command ('c') is required"
			sendErr := sendMessage(*conn.ws, websocketErrorMessage{ErrorCode: errCode, Info: &errMsg})
			if sendErr != nil {
				break
			}
		} else if message.Command == "ping" {
			command := "pong"
			sendErr := sendMessage(*conn.ws, websocketMessage{Command: command})
			if sendErr != nil {
				break
			}
		} else if message.Command == "pong" {
			// Don't need to do anything, just here to prevent invalid command error
			conn.pongsRecieved++
		} else if triggersRegistrationFlow(&message) {
			regErr := registrationFlow(&conn, message)
			if regErr != nil {
				break
			}
		} else if triggersAuthenticationFlow(&message) {
			authErr := authenticationFlow(&conn, message)
			if authErr != nil {
				break
			}
		} else {
			errCode := uint(0)
			errMsg := fmt.Sprintf("Invalid command '%s'", message.Command)
			sendErr := sendMessage(*conn.ws, websocketErrorMessage{ErrorCode: errCode, Info: &errMsg})
			if sendErr != nil {
				break
			}
		}
	}
}
