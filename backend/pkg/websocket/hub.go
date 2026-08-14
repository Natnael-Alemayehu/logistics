package websocket

import (
	"sync"

	"github.com/rs/zerolog"
)

type Hub struct {
	Clients    map[*Client]bool
	Register   chan *Client
	Unregister chan *Client
	Broadcast  chan *BroadcastMessage
	Rooms      map[string]*Room
	mutex      sync.RWMutex
	logger     zerolog.Logger
}

type BroadcastMessage struct {
	RoomID  string
	Message []byte
	Exclude *Client
}

func NewHub(logger zerolog.Logger) *Hub {
	return &Hub{
		Clients:    make(map[*Client]bool),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Broadcast:  make(chan *BroadcastMessage, 512),
		Rooms:      make(map[string]*Room),
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

		case msg := <-h.Broadcast:
			h.broadcastToRoom(msg)
		}
	}
}

func (h *Hub) registerClient(client *Client) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	h.Clients[client] = true
	roomID := GetTenantRoomID(client.TenantID)
	room := h.getOrCreateRoom(roomID)
	room.Subscribe(client)

	h.logger.Info().
		Str("user_id", client.UserID).
		Str("tenant_id", client.TenantID).
		Str("room", roomID).
		Msg("WebSocket client connected")
}

func (h *Hub) unregisterClient(client *Client) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if _, ok := h.Clients[client]; ok {
		delete(h.Clients, client)
		close(client.Send)

		for roomID := range client.Rooms {
			if room, exists := h.Rooms[roomID]; exists {
				room.Unsubscribe(client)
				if room.IsEmpty() {
					delete(h.Rooms, roomID)
				}
			}
		}

		h.logger.Info().
			Str("user_id", client.UserID).
			Str("tenant_id", client.TenantID).
			Msg("WebSocket client disconnected")
	}
}

func (h *Hub) getOrCreateRoom(roomID string) *Room {
	if room, exists := h.Rooms[roomID]; exists {
		return room
	}
	room := NewRoom(roomID)
	h.Rooms[roomID] = room
	return room
}

func (h *Hub) broadcastToRoom(msg *BroadcastMessage) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	room, exists := h.Rooms[msg.RoomID]
	if !exists {
		return
	}

	for client := range room.Clients {
		if msg.Exclude != nil && client == msg.Exclude {
			continue
		}
		select {
		case client.Send <- msg.Message:
		default:
			h.logger.Warn().
				Str("user_id", client.UserID).
				Msg("Client send buffer full, skipping message")
		}
	}
}

func (h *Hub) BroadcastToTenant(tenantID string, message []byte) {
	roomID := GetTenantRoomID(tenantID)
	h.Broadcast <- &BroadcastMessage{
		RoomID:  roomID,
		Message: message,
	}
}

func (h *Hub) BroadcastToDriver(driverID string, message []byte) {
	roomID := GetDriverRoomID(driverID)
	h.Broadcast <- &BroadcastMessage{
		RoomID:  roomID,
		Message: message,
	}
}

func (h *Hub) BroadcastToShipment(shipmentID string, message []byte) {
	roomID := GetShipmentRoomID(shipmentID)
	h.Broadcast <- &BroadcastMessage{
		RoomID:  roomID,
		Message: message,
	}
}

func (h *Hub) BroadcastAll(message []byte) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	for client := range h.Clients {
		select {
		case client.Send <- message:
		default:
		}
	}
}

func (h *Hub) SubscribeToRoom(client *Client, roomID string) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	room := h.getOrCreateRoom(roomID)
	room.Subscribe(client)
}

func (h *Hub) UnsubscribeFromRoom(client *Client, roomID string) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if room, exists := h.Rooms[roomID]; exists {
		room.Unsubscribe(client)
		if room.IsEmpty() {
			delete(h.Rooms, roomID)
		}
	}
}

func (h *Hub) ClientCount() int {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	return len(h.Clients)
}

func (h *Hub) RoomCount() int {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	return len(h.Rooms)
}

func (h *Hub) TenantClientCount(tenantID string) int {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	roomID := GetTenantRoomID(tenantID)
	if room, exists := h.Rooms[roomID]; exists {
		return room.ClientCount()
	}
	return 0
}
