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
				age := time.Since(conn.latestMessage)
				heartbeat_age := time.Since(conn.latestHeartbeat)
				if age >= conn.handler.config.Heartbeat.KillDelay {
					errCode := uint(1)
					errMsg := "Hearbeat missed"
					sendMessage(conn.ws, websocketErrorMessage{ErrorCode: errCode, Info: &errMsg})
					conn.close()
					logger.Info(fmt.Sprintf(
						"Disconnected %d, heartbeat missed. %.2f%% response rate (%d/%d)",
						conn.connectionID,
						float32(conn.pongsRecieved)/float32(conn.pingsSent)*100,
						conn.pongsRecieved,
						conn.pingsSent,
					))
				} else if age >= conn.handler.config.Heartbeat.Delay && heartbeat_age >= conn.handler.config.Heartbeat.Interval {
					command := "ping"
					sendMessage(conn.ws, websocketMessage{Command: command})
					conn.pingsSent++
					conn.latestHeartbeat = time.Now()
					logger.Info(fmt.Sprintf("Send heartbeat to %d", conn.connectionID))
				}
			}
		}
	}()
}

func (conn *websocketConnection) stopHeartbeatMonitor() {
	if conn.hearbeat_cancel != nil {
		conn.hearbeat_cancel()
		conn.hearbeat_cancel = nil
	}
}
