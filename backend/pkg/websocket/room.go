package websocket

import (
	"sync"
)

type Room struct {
	Name      string
	Clients   map[*Client]bool
	Broadcast chan []byte
	mutex     sync.RWMutex
}

func NewRoom(name string) *Room {
	return &Room{
		Name:      name,
		Clients:   make(map[*Client]bool),
		Broadcast: make(chan []byte, 256),
	}
}

func (r *Room) Subscribe(client *Client) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.Clients[client] = true
	client.Rooms[r.Name] = true
}

func (r *Room) Unsubscribe(client *Client) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if _, ok := r.Clients[client]; ok {
		delete(r.Clients, client)
		delete(client.Rooms, r.Name)
	}
}

func (r *Room) BroadcastMessage(message []byte) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	for client := range r.Clients {
		select {
		case client.Send <- message:
		default:
			close(client.Send)
			delete(r.Clients, client)
		}
	}
}

func (r *Room) ClientCount() int {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return len(r.Clients)
}

func (r *Room) IsEmpty() bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return len(r.Clients) == 0
}

func GetTenantRoomID(tenantID string) string {
	return "tenant:" + tenantID
}

func GetDriverRoomID(driverID string) string {
	return "driver:" + driverID
}

func GetShipmentRoomID(shipmentID string) string {
	return "shipment:" + shipmentID
}
