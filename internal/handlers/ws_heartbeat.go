package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/CLDWare/schoolbox-backend/pkg/logger"
)

func (conn *websocketConnection) startHeartbeatMonitor() {
	ctx, cancel := context.WithCancel(context.Background())
	conn.hearbeat_cancel = cancel

	go func() {
		ticker := time.NewTicker(conn.handler.config.Heartbeat.CheckInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				conn.mu.RLock()
				age := time.Since(conn.latestMessage)
				heartbeat_age := time.Since(conn.latestHeartbeat)
				conn.mu.RUnlock()
				if age >= conn.handler.config.Heartbeat.KillDelay {
					errCode := 1
					errMsg := "Hearbeat missed"
					sendMessage(conn.ws, websocketErrorMessage{ErrorCode: errCode, Info: &errMsg}) // heartbeat missed
					conn.close()
					logger.Info(fmt.Sprintf(
						"Disconnected %d, heartbeat missed. %.2f%% response rate (%d/%d)",
						conn.connectionID,
						float32(conn.pongsReceived)/float32(conn.pingsSent)*100,
						conn.pongsReceived,
						conn.pingsSent,
					))
				} else if age >= conn.handler.config.Heartbeat.Delay && heartbeat_age >= conn.handler.config.Heartbeat.Interval {
					command := "ping"
					sendMessage(conn.ws, websocketMessage{Command: command})
					conn.mu.Lock()
					conn.pingsSent++
					conn.latestHeartbeat = time.Now()
					conn.mu.Unlock()
					logger.Info(fmt.Sprintf("Send heartbeat to %d", conn.connectionID))
				}
			}
		}
	}()
}

func (conn *websocketConnection) stopHeartbeatMonitor() {
	conn.mu.Lock()
	cancel_func := conn.hearbeat_cancel
	conn.hearbeat_cancel = nil
	conn.mu.Unlock()

	if cancel_func != nil {
		cancel_func()
	}
}
