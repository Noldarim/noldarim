// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/noldarim/noldarim/internal/protocol"

	"github.com/gorilla/websocket"
)

const (
	// WebSocket limits
	maxMessageSize  = 4096
	maxFilters      = 50
	pongWait        = 60 * time.Second
	pingPeriod      = (pongWait * 9) / 10
	writeWait       = 10 * time.Second
	maxClients      = 1000
)

// newUpgrader creates a WebSocket upgrader that respects the configured allowed
// origins. When allowedOrigins is empty the upgrader accepts any origin
// (localhost development mode). When set, only those origins are permitted.
func newUpgrader(allowedOrigins []string) websocket.Upgrader {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = struct{}{}
	}

	return websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			if len(allowed) == 0 {
				return true
			}
			origin := r.Header.Get("Origin")
			_, ok := allowed[origin]
			return ok
		},
	}
}

// SubscriptionFilter determines which events a WebSocket client receives.
type SubscriptionFilter struct {
	ProjectID string `json:"project_id,omitempty"`
	TaskID    string `json:"task_id,omitempty"`
	RunID     string `json:"run_id,omitempty"`
}

// wsClient represents a single connected WebSocket client.
type wsClient struct {
	conn    *websocket.Conn
	send    chan []byte
	filters []SubscriptionFilter
	mu      sync.RWMutex
}

// ClientRegistry manages all connected WebSocket clients.
type ClientRegistry struct {
	mu      sync.RWMutex
	clients map[*wsClient]struct{}
}

// NewClientRegistry creates a new client registry.
func NewClientRegistry() *ClientRegistry {
	return &ClientRegistry{
		clients: make(map[*wsClient]struct{}),
	}
}

// Broadcast sends an event to all clients whose filters match.
func (r *ClientRegistry) Broadcast(event protocol.Event) {
	data, err := marshalEvent(event)
	if err != nil {
		getLog().Error().Err(err).Msg("Failed to marshal event for WebSocket broadcast")
		return
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	for c := range r.clients {
		if c.matchesAny(event) {
			select {
			case c.send <- data:
			default:
				// client too slow, skip
				getLog().Warn().Msg("Dropping event for slow WebSocket client")
			}
		}
	}
}

func (r *ClientRegistry) add(c *wsClient) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.clients) >= maxClients {
		return false
	}
	r.clients[c] = struct{}{}
	return true
}

func (r *ClientRegistry) remove(c *wsClient) {
	r.mu.Lock()
	delete(r.clients, c)
	r.mu.Unlock()
}

// matchesAny returns true if the event matches any of the client's filters,
// or if the client has no filters (receives everything).
func (c *wsClient) matchesAny(event protocol.Event) bool {
	c.mu.RLock()
	if len(c.filters) == 0 {
		c.mu.RUnlock()
		return true
	}
	// Copy to avoid reading from a slice that could be modified after unlock
	filters := make([]SubscriptionFilter, len(c.filters))
	copy(filters, c.filters)
	c.mu.RUnlock()

	projectID, taskID, runID := extractEventIDs(event)

	for _, f := range filters {
		if f.ProjectID != "" && f.ProjectID != projectID {
			continue
		}
		if f.TaskID != "" && f.TaskID != taskID {
			continue
		}
		if f.RunID != "" && f.RunID != runID {
			continue
		}
		return true
	}
	return false
}

// projectScoped, taskScoped, and runScoped allow events to declare their IDs
// without requiring this file to enumerate every event type.
type projectScoped interface {
	GetProjectID() string
}

type taskScoped interface {
	GetTaskID() string
}

type runScoped interface {
	GetRunID() string
}

// extractEventIDs extracts project, task, and run IDs from events.
// Events that implement projectScoped/taskScoped/runScoped are handled automatically.
// Special cases (e.g. ProjectCreatedEvent) still need explicit handling.
func extractEventIDs(event protocol.Event) (projectID, taskID, runID string) {
	if ps, ok := event.(projectScoped); ok {
		projectID = ps.GetProjectID()
	}
	if ts, ok := event.(taskScoped); ok {
		taskID = ts.GetTaskID()
	}
	if rs, ok := event.(runScoped); ok {
		runID = rs.GetRunID()
	}
	// Special case: ProjectCreatedEvent carries ID inside nested struct
	if e, ok := event.(protocol.ProjectCreatedEvent); ok && e.Project != nil {
		projectID = e.Project.ID
	}
	return projectID, taskID, runID
}

// wsMessage is the envelope for client → server WebSocket messages.
type wsMessage struct {
	Type    string             `json:"type"`    // "subscribe" or "unsubscribe"
	Filters SubscriptionFilter `json:"filters"` // single filter per message
}

// wsOutMessage is the envelope for server → client WebSocket messages.
type wsOutMessage struct {
	Type      string      `json:"type"`                 // "event" or "error"
	EventType string      `json:"event_type,omitempty"` // Go type name
	Payload   interface{} `json:"payload,omitempty"`
	Message   string      `json:"message,omitempty"`
}

func marshalEvent(event protocol.Event) ([]byte, error) {
	out := wsOutMessage{
		Type:      "event",
		EventType: fmt.Sprintf("%T", event),
		Payload:   event,
	}
	return json.Marshal(out)
}

// HandleWebSocket upgrades an HTTP connection and manages the client lifecycle.
func HandleWebSocket(registry *ClientRegistry, allowedOrigins []string) http.HandlerFunc {
	upgrader := newUpgrader(allowedOrigins)

	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			getLog().Error().Err(err).Msg("WebSocket upgrade failed")
			return
		}

		client := &wsClient{
			conn: conn,
			send: make(chan []byte, 64),
		}
		if !registry.add(client) {
			getLog().Warn().Msg("WebSocket connection limit reached")
			conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseTryAgainLater, "too many connections"))
			conn.Close()
			return
		}
		getLog().Info().Str("remote", r.RemoteAddr).Msg("WebSocket client connected")

		go client.writePump()
		client.readPump(registry)
	}
}

func (c *wsClient) readPump(registry *ClientRegistry) {
	defer func() {
		registry.remove(c)
		close(c.send) // signals writePump to exit
		c.conn.Close()
		getLog().Info().Msg("WebSocket client disconnected")
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				getLog().Error().Err(err).Msg("WebSocket read error")
			}
			return
		}

		var msg wsMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			getLog().Warn().Err(err).Msg("Invalid WebSocket message")
			continue
		}

		c.mu.Lock()
		switch msg.Type {
		case "subscribe":
			if len(c.filters) >= maxFilters {
				getLog().Warn().Msg("WebSocket client hit max filter limit")
			} else {
				c.filters = append(c.filters, msg.Filters)
				getLog().Debug().
					Str("project_id", msg.Filters.ProjectID).
					Str("task_id", msg.Filters.TaskID).
					Msg("WebSocket client subscribed")
			}
		case "unsubscribe":
			c.filters = removeFilter(c.filters, msg.Filters)
			getLog().Debug().Msg("WebSocket client unsubscribed")
		}
		c.mu.Unlock()
	}
}

func (c *wsClient) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case data, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Channel closed by readPump, send close frame.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				getLog().Error().Err(err).Msg("WebSocket write error")
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func removeFilter(filters []SubscriptionFilter, target SubscriptionFilter) []SubscriptionFilter {
	result := make([]SubscriptionFilter, 0, len(filters))
	for _, f := range filters {
		if f.ProjectID == target.ProjectID && f.TaskID == target.TaskID && f.RunID == target.RunID {
			continue
		}
		result = append(result, f)
	}
	return result
}
