package websocket

import (
	"io"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestLogger() zerolog.Logger {
	return zerolog.New(io.Discard)
}

func newTestClient(t *testing.T, hub *Hub, tenantID, userID string) *Client {
	return NewClient(hub, nil, tenantID, userID, "driver")
}

func TestNewRoom(t *testing.T) {
	t.Run("creates room with correct name", func(t *testing.T) {
		roomName := "tenant:123"
		room := NewRoom(roomName)

		require.NotNil(t, room)
		assert.Equal(t, roomName, room.Name)
		assert.NotNil(t, room.Clients)
		assert.Empty(t, room.Clients)
		assert.NotNil(t, room.Broadcast)
	})
}

func TestRoom_Subscribe(t *testing.T) {
	t.Run("adds client to room", func(t *testing.T) {
		room := NewRoom("tenant:123")
		hub := NewHub(newTestLogger())
		client := newTestClient(t, hub, "tenant-123", "user-1")

		room.Subscribe(client)

		assert.True(t, room.Clients[client])
		assert.True(t, client.Rooms[room.Name])
		assert.Equal(t, 1, room.ClientCount())
	})

	t.Run("adds multiple clients to room", func(t *testing.T) {
		room := NewRoom("tenant:123")
		hub := NewHub(newTestLogger())
		client1 := newTestClient(t, hub, "tenant-123", "user-1")
		client2 := newTestClient(t, hub, "tenant-123", "user-2")

		room.Subscribe(client1)
		room.Subscribe(client2)

		assert.Equal(t, 2, room.ClientCount())
		assert.True(t, room.Clients[client1])
		assert.True(t, room.Clients[client2])
	})
}

func TestRoom_Unsubscribe(t *testing.T) {
	t.Run("removes client from room", func(t *testing.T) {
		room := NewRoom("tenant:123")
		hub := NewHub(newTestLogger())
		client := newTestClient(t, hub, "tenant-123", "user-1")

		room.Subscribe(client)
		require.Equal(t, 1, room.ClientCount())

		room.Unsubscribe(client)

		assert.False(t, room.Clients[client])
		assert.False(t, client.Rooms[room.Name])
		assert.Equal(t, 0, room.ClientCount())
	})

	t.Run("handles unsubscribing non-existent client", func(t *testing.T) {
		room := NewRoom("tenant:123")
		hub := NewHub(newTestLogger())
		client := newTestClient(t, hub, "tenant-123", "user-1")

		room.Unsubscribe(client)

		assert.Equal(t, 0, room.ClientCount())
	})

	t.Run("removes only specified client", func(t *testing.T) {
		room := NewRoom("tenant:123")
		hub := NewHub(newTestLogger())
		client1 := newTestClient(t, hub, "tenant-123", "user-1")
		client2 := newTestClient(t, hub, "tenant-123", "user-2")

		room.Subscribe(client1)
		room.Subscribe(client2)
		require.Equal(t, 2, room.ClientCount())

		room.Unsubscribe(client1)

		assert.Equal(t, 1, room.ClientCount())
		assert.False(t, room.Clients[client1])
		assert.True(t, room.Clients[client2])
	})
}

func TestRoom_BroadcastMessage(t *testing.T) {
	t.Run("sends message to all clients", func(t *testing.T) {
		room := NewRoom("tenant:123")
		hub := NewHub(newTestLogger())
		client1 := newTestClient(t, hub, "tenant-123", "user-1")
		client2 := newTestClient(t, hub, "tenant-123", "user-2")

		room.Subscribe(client1)
		room.Subscribe(client2)

		message := []byte(`{"type":"test"}`)
		room.BroadcastMessage(message)

		msg1 := <-client1.Send
		msg2 := <-client2.Send

		assert.Equal(t, message, msg1)
		assert.Equal(t, message, msg2)
	})

	t.Run("handles empty room", func(t *testing.T) {
		room := NewRoom("tenant:123")

		message := []byte(`{"type":"test"}`)
		assert.NotPanics(t, func() {
			room.BroadcastMessage(message)
		})
	})

	t.Run("closes client send channel when buffer full", func(t *testing.T) {
		room := NewRoom("tenant:123")
		hub := NewHub(newTestLogger())

		smallBufferClient := &Client{
			Hub:      hub,
			Send:     make(chan []byte, 1),
			TenantID: "tenant-123",
			UserID:   "user-1",
			Rooms:    make(map[string]bool),
		}

		room.Subscribe(smallBufferClient)

		for i := 0; i < 10; i++ {
			room.BroadcastMessage([]byte(`{"type":"test"}`))
		}

		_, ok := room.Clients[smallBufferClient]
		assert.False(t, ok, "client should be removed from room when send buffer is full")
	})
}

func TestRoom_ClientCount(t *testing.T) {
	t.Run("returns correct count", func(t *testing.T) {
		room := NewRoom("tenant:123")
		hub := NewHub(newTestLogger())

		assert.Equal(t, 0, room.ClientCount())

		client1 := newTestClient(t, hub, "tenant-123", "user-1")
		room.Subscribe(client1)
		assert.Equal(t, 1, room.ClientCount())

		client2 := newTestClient(t, hub, "tenant-123", "user-2")
		room.Subscribe(client2)
		assert.Equal(t, 2, room.ClientCount())

		room.Unsubscribe(client1)
		assert.Equal(t, 1, room.ClientCount())
	})
}

func TestRoom_IsEmpty(t *testing.T) {
	t.Run("returns true for empty room", func(t *testing.T) {
		room := NewRoom("tenant:123")
		assert.True(t, room.IsEmpty())
	})

	t.Run("returns false for non-empty room", func(t *testing.T) {
		room := NewRoom("tenant:123")
		hub := NewHub(newTestLogger())
		client := newTestClient(t, hub, "tenant-123", "user-1")

		room.Subscribe(client)
		assert.False(t, room.IsEmpty())
	})

	t.Run("returns true after all clients unsubscribe", func(t *testing.T) {
		room := NewRoom("tenant:123")
		hub := NewHub(newTestLogger())
		client := newTestClient(t, hub, "tenant-123", "user-1")

		room.Subscribe(client)
		room.Unsubscribe(client)
		assert.True(t, room.IsEmpty())
	})
}

func TestGetTenantRoomID(t *testing.T) {
	result := GetTenantRoomID("tenant-123")
	assert.Equal(t, "tenant:tenant-123", result)
}

func TestGetDriverRoomID(t *testing.T) {
	result := GetDriverRoomID("driver-456")
	assert.Equal(t, "driver:driver-456", result)
}

func TestGetShipmentRoomID(t *testing.T) {
	result := GetShipmentRoomID("shipment-789")
	assert.Equal(t, "shipment:shipment-789", result)
}
