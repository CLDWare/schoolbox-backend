package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/CLDWare/schoolbox-backend/config"
	models "github.com/CLDWare/schoolbox-backend/pkg/db"
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
	mu               sync.RWMutex
}

func (h *WebsocketHandler) addConnection(conn *websocketConnection) {
	conn.mu.Lock()
	conn.handler = h
	h.mu.Lock()
	conn.connectionID = h.nextID
	conn.mu.Unlock()
	h.connections[h.nextID] = conn
	h.nextID = h.nextID + 1
	h.mu.Unlock()

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
	state           uint // 0 none;1 registering;2 authenticating;3 authenticated;4 active_session;
	stateFlow       any
	connectedAt     time.Time
	latestMessage   time.Time
	hearbeat_cancel context.CancelFunc
	latestHeartbeat time.Time
	pingsSent       uint
	pongsReceived   uint
	mu              sync.RWMutex
}

func (conn *websocketConnection) close() error {
	if err := conn.ws.Close(); err != nil {
		logger.Err("Error closing websocket:", err)
	}
	conn.stopHeartbeatMonitor()

	conn.handler.mu.Lock()
	defer conn.handler.mu.Unlock()
	delete(conn.handler.connections, conn.connectionID)
	if conn.deviceID != nil {
		delete(conn.handler.connectedDevices, *conn.deviceID)
		logger.Info(fmt.Sprintf("Closed connection %d, device %d", conn.connectionID, *conn.deviceID))
	} else {
		logger.Info(fmt.Sprintf("Closed connection %d", conn.connectionID))
	}
	regFlowData, ok := conn.stateFlow.(registrationFlowData)
	if ok {
		delete(conn.handler.registrationPins, regFlowData.pin)
	}
	return nil
}

type websocketMessage struct {
	Command string         `json:"c,omitempty"`
	Data    map[string]any `json:"d,omitempty"`
}

type websocketErrorMessage struct {
	ErrorCode int     `json:"e,omitempty"`
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

func sendMessage(ws *websocket.Conn, msg any) error {
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

// InitialiseWebscoket
//
// @Summary		Open a connection to the device websocket API
// @Description	Open a websocket connection used by devices to communicate with the server.
// @Description Devices get notified about session changes and send votes via this connection.
// @Tags			device_websocket
// @Accept       json
// @Produce      json
// @Success      101 {string} string "Switching Protocols (WebSocket Upgrade)"
// @Failure      400 {string} string "Bad Request"
// @Router       /ws [get]
func (h *WebsocketHandler) InitialiseWebsocket(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Err(err)
		return
	}
	ctx := context.Background()

	conn := websocketConnection{
		handler:       h,
		ws:            ws,
		db:            h.db,
		connectedAt:   time.Now(),
		latestMessage: time.Now(),
	}
	h.addConnection(&conn)
	conn.startHeartbeatMonitor()
	defer conn.close()
	logger.Info(fmt.Sprintf("New connection %d", conn.connectionID))

	for {
		// Read message from client
		_, msg, err := ws.ReadMessage()
		if err != nil {
			logger.Err("read:", err)
			break
		}
		conn.mu.Lock()
		conn.latestMessage = time.Now()
		conn.mu.Unlock()
		if conn.deviceID != nil {
			gorm.G[models.Device](h.db).Where("id = ?", conn.deviceID).Update(ctx, "LastSeen", time.Now())
		}

		var message websocketMessage
		err = json.Unmarshal(msg, &message)
		if err != nil {
			logger.Err("Invalid JSON:", err)
			errCode := 0
			errMsg := err.Error()
			sendErr := sendMessage(conn.ws, websocketErrorMessage{ErrorCode: errCode, Info: &errMsg}) // bad request
			if sendErr != nil {
				break
			}
			continue
		}

		logger.Info(fmt.Sprintf("Received: %s", msg))

		if message.Command == "" {
			errCode := 0
			errMsg := "A command ('c') is required"
			sendErr := sendMessage(conn.ws, websocketErrorMessage{ErrorCode: errCode, Info: &errMsg}) // bad request
			if sendErr != nil {
				break
			}
		} else if message.Command == "ping" {
			command := "pong"
			sendErr := sendMessage(conn.ws, websocketMessage{Command: command})
			if sendErr != nil {
				break
			}
		} else if message.Command == "pong" {
			// Don't need to do anything, just here to prevent invalid command error
			conn.mu.Lock()
			conn.pongsReceived++
			conn.mu.Unlock()
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
		} else if triggersSessionFlow(&message) {
			sessionErr := sessionFlow(&conn, message)
			if sessionErr != nil {
				break
			}
		} else {
			errCode := 0
			errMsg := fmt.Sprintf("Invalid command '%s'", message.Command)
			sendErr := sendMessage(conn.ws, websocketErrorMessage{ErrorCode: errCode, Info: &errMsg}) // bad request
			if sendErr != nil {
				break
			}
		}
	}
}
