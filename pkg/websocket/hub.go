package websocket

import (
	"sync"

	"github.com/rs/zerolog"

	"github.com/natnael-alemayehu/kiflet/pkg/metrics"
)

// Hub sends each company's live updates to that company's open dashboards,
// and to nobody else's.
type Hub struct {
	Register   chan *Client
	Unregister chan *Client

	broadcast chan tenantMessage
	mutex     sync.RWMutex
	tenants   map[string]map[*Client]bool // each company's clients, by tenant ID
	logger    zerolog.Logger
}

type tenantMessage struct {
	tenantID string
	message  []byte
}

func NewHub(logger zerolog.Logger) *Hub {
	return &Hub{
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		broadcast:  make(chan tenantMessage, 512),
		tenants:    make(map[string]map[*Client]bool),
		logger:     logger,
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.registerClient(client)

		case client := <-h.Unregister:
			h.unregisterClient(client)

		case msg := <-h.broadcast:
			h.send(msg)
		}
	}
}

func (h *Hub) registerClient(client *Client) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	clients := h.tenants[client.TenantID]
	if clients == nil {
		clients = make(map[*Client]bool)
		h.tenants[client.TenantID] = clients
	}
	if clients[client] {
		return
	}
	clients[client] = true
	metrics.WebSocketConnections.Inc()

	h.logger.Info().
		Str("user_id", client.UserID).
		Str("tenant_id", client.TenantID).
		Msg("WebSocket client connected")
}

func (h *Hub) unregisterClient(client *Client) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	clients := h.tenants[client.TenantID]
	if !clients[client] {
		return
	}
	delete(clients, client)
	if len(clients) == 0 {
		delete(h.tenants, client.TenantID)
	}
	close(client.Send)
	metrics.WebSocketConnections.Dec()

	h.logger.Info().
		Str("user_id", client.UserID).
		Str("tenant_id", client.TenantID).
		Msg("WebSocket client disconnected")
}

// send hands a message to every client of one company. A client whose
// buffer is full misses the message: the hub never waits for a slow browser.
func (h *Hub) send(msg tenantMessage) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	for client := range h.tenants[msg.tenantID] {
		select {
		case client.Send <- msg.message:
		default:
			h.logger.Warn().
				Str("user_id", client.UserID).
				Msg("Client send buffer full, skipping message")
		}
	}
}

// BroadcastToTenant queues a message for every open dashboard of a company.
func (h *Hub) BroadcastToTenant(tenantID string, message []byte) {
	h.broadcast <- tenantMessage{tenantID: tenantID, message: message}
}

// ClientCount is the number of open connections, across all companies.
func (h *Hub) ClientCount() int {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	n := 0
	for _, clients := range h.tenants {
		n += len(clients)
	}
	return n
}
