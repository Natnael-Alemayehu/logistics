package websocket

import (
	"io"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestHubLogger() zerolog.Logger {
	return zerolog.New(io.Discard)
}

func newTestHubClient(t *testing.T, hub *Hub, tenantID, userID string) *Client {
	return NewClient(hub, nil, tenantID, userID, "driver")
}

func TestNewHub(t *testing.T) {
	t.Run("creates hub with correct initialization", func(t *testing.T) {
		logger := newTestHubLogger()
		hub := NewHub(logger)

		require.NotNil(t, hub)
		assert.NotNil(t, hub.Clients)
		assert.Empty(t, hub.Clients)
		assert.NotNil(t, hub.Register)
		assert.NotNil(t, hub.Unregister)
		assert.NotNil(t, hub.Broadcast)
		assert.NotNil(t, hub.Rooms)
		assert.Empty(t, hub.Rooms)
	})
}

func TestHub_registerClient(t *testing.T) {
	t.Run("adds client and creates room", func(t *testing.T) {
		hub := NewHub(newTestHubLogger())
		client := newTestHubClient(t, hub, "tenant-123", "user-1")

		hub.registerClient(client)

		assert.True(t, hub.Clients[client])
		assert.Equal(t, 1, hub.ClientCount())

		roomID := GetTenantRoomID("tenant-123")
		assert.NotNil(t, hub.Rooms[roomID])
		assert.Equal(t, 1, hub.Rooms[roomID].ClientCount())
	})

	t.Run("adds multiple clients to same tenant room", func(t *testing.T) {
		hub := NewHub(newTestHubLogger())
		client1 := newTestHubClient(t, hub, "tenant-123", "user-1")
		client2 := newTestHubClient(t, hub, "tenant-123", "user-2")

		hub.registerClient(client1)
		hub.registerClient(client2)

		assert.Equal(t, 2, hub.ClientCount())

		roomID := GetTenantRoomID("tenant-123")
		assert.Equal(t, 2, hub.Rooms[roomID].ClientCount())
	})

	t.Run("creates separate rooms for different tenants", func(t *testing.T) {
		hub := NewHub(newTestHubLogger())
		client1 := newTestHubClient(t, hub, "tenant-123", "user-1")
		client2 := newTestHubClient(t, hub, "tenant-456", "user-2")

		hub.registerClient(client1)
		hub.registerClient(client2)

		assert.Equal(t, 2, hub.ClientCount())
		assert.Equal(t, 2, hub.RoomCount())

		room1ID := GetTenantRoomID("tenant-123")
		room2ID := GetTenantRoomID("tenant-456")
		assert.Equal(t, 1, hub.Rooms[room1ID].ClientCount())
		assert.Equal(t, 1, hub.Rooms[room2ID].ClientCount())
	})
}

func TestHub_unregisterClient(t *testing.T) {
	t.Run("removes client and cleans up rooms", func(t *testing.T) {
		hub := NewHub(newTestHubLogger())
		client := newTestHubClient(t, hub, "tenant-123", "user-1")
		hub.registerClient(client)
		require.Equal(t, 1, hub.ClientCount())

		hub.unregisterClient(client)

		assert.False(t, hub.Clients[client])
		assert.Equal(t, 0, hub.ClientCount())

		roomID := GetTenantRoomID("tenant-123")
		_, roomExists := hub.Rooms[roomID]
		assert.False(t, roomExists, "empty room should be cleaned up")
	})

	t.Run("handles unregistering non-existent client", func(t *testing.T) {
		hub := NewHub(newTestHubLogger())
		client := newTestHubClient(t, hub, "tenant-123", "user-1")

		assert.NotPanics(t, func() {
			hub.unregisterClient(client)
		})
	})

	t.Run("keeps room if other clients remain", func(t *testing.T) {
		hub := NewHub(newTestHubLogger())
		client1 := newTestHubClient(t, hub, "tenant-123", "user-1")
		client2 := newTestHubClient(t, hub, "tenant-123", "user-2")

		hub.registerClient(client1)
		hub.registerClient(client2)
		require.Equal(t, 2, hub.ClientCount())

		hub.unregisterClient(client1)

		assert.Equal(t, 1, hub.ClientCount())

		roomID := GetTenantRoomID("tenant-123")
		assert.NotNil(t, hub.Rooms[roomID])
		assert.Equal(t, 1, hub.Rooms[roomID].ClientCount())
	})
}

func TestHub_BroadcastToTenant(t *testing.T) {
	t.Run("sends to correct room", func(t *testing.T) {
		hub := NewHub(newTestHubLogger())
		client1 := newTestHubClient(t, hub, "tenant-123", "user-1")
		client2 := newTestHubClient(t, hub, "tenant-456", "user-2")

		hub.registerClient(client1)
		hub.registerClient(client2)

		message := []byte(`{"type":"test"}`)
		roomID := GetTenantRoomID("tenant-123")
		hub.broadcastToRoom(&BroadcastMessage{RoomID: roomID, Message: message})

		msg := <-client1.Send
		assert.Equal(t, message, msg)

		select {
		case <-client2.Send:
			t.Fatal("client2 should not receive message")
		case <-time.After(100 * time.Millisecond):
		}
	})
}

func TestHub_BroadcastToDriver(t *testing.T) {
	t.Run("sends to correct room", func(t *testing.T) {
		hub := NewHub(newTestHubLogger())
		client := newTestHubClient(t, hub, "tenant-123", "user-1")

		hub.registerClient(client)

		driverRoomID := GetDriverRoomID("driver-123")
		hub.SubscribeToRoom(client, driverRoomID)

		message := []byte(`{"type":"driver_update"}`)
		hub.broadcastToRoom(&BroadcastMessage{RoomID: driverRoomID, Message: message})

		msg := <-client.Send
		assert.Equal(t, message, msg)
	})
}

func TestHub_BroadcastToShipment(t *testing.T) {
	t.Run("sends to correct room", func(t *testing.T) {
		hub := NewHub(newTestHubLogger())
		client := newTestHubClient(t, hub, "tenant-123", "user-1")

		hub.registerClient(client)

		shipmentRoomID := GetShipmentRoomID("shipment-789")
		hub.SubscribeToRoom(client, shipmentRoomID)

		message := []byte(`{"type":"shipment_update"}`)
		hub.broadcastToRoom(&BroadcastMessage{RoomID: shipmentRoomID, Message: message})

		msg := <-client.Send
		assert.Equal(t, message, msg)
	})
}

func TestHub_BroadcastAll(t *testing.T) {
	t.Run("sends to all clients", func(t *testing.T) {
		hub := NewHub(newTestHubLogger())
		client1 := newTestHubClient(t, hub, "tenant-123", "user-1")
		client2 := newTestHubClient(t, hub, "tenant-456", "user-2")
		client3 := newTestHubClient(t, hub, "tenant-789", "user-3")

		hub.registerClient(client1)
		hub.registerClient(client2)
		hub.registerClient(client3)

		message := []byte(`{"type":"broadcast"}`)
		hub.BroadcastAll(message)

		msg1 := <-client1.Send
		msg2 := <-client2.Send
		msg3 := <-client3.Send

		assert.Equal(t, message, msg1)
		assert.Equal(t, message, msg2)
		assert.Equal(t, message, msg3)
	})

	t.Run("handles empty hub", func(t *testing.T) {
		hub := NewHub(newTestHubLogger())

		message := []byte(`{"type":"broadcast"}`)
		assert.NotPanics(t, func() {
			hub.BroadcastAll(message)
		})
	})
}

func TestHub_SubscribeToRoom(t *testing.T) {
	t.Run("adds client to room", func(t *testing.T) {
		hub := NewHub(newTestHubLogger())
		client := newTestHubClient(t, hub, "tenant-123", "user-1")

		roomID := "custom-room"
		hub.SubscribeToRoom(client, roomID)

		assert.NotNil(t, hub.Rooms[roomID])
		assert.True(t, client.Rooms[roomID])
		assert.Equal(t, 1, hub.Rooms[roomID].ClientCount())
	})

	t.Run("creates room if not exists", func(t *testing.T) {
		hub := NewHub(newTestHubLogger())
		client := newTestHubClient(t, hub, "tenant-123", "user-1")

		roomID := "new-room"
		_, exists := hub.Rooms[roomID]
		assert.False(t, exists)

		hub.SubscribeToRoom(client, roomID)

		assert.NotNil(t, hub.Rooms[roomID])
	})

	t.Run("adds to existing room", func(t *testing.T) {
		hub := NewHub(newTestHubLogger())
		client1 := newTestHubClient(t, hub, "tenant-123", "user-1")
		client2 := newTestHubClient(t, hub, "tenant-123", "user-2")

		roomID := "shared-room"
		hub.SubscribeToRoom(client1, roomID)
		hub.SubscribeToRoom(client2, roomID)

		assert.Equal(t, 1, hub.RoomCount())
		assert.Equal(t, 2, hub.Rooms[roomID].ClientCount())
	})
}

func TestHub_UnsubscribeFromRoom(t *testing.T) {
	t.Run("removes client from room", func(t *testing.T) {
		hub := NewHub(newTestHubLogger())
		client := newTestHubClient(t, hub, "tenant-123", "user-1")

		roomID := "test-room"
		hub.SubscribeToRoom(client, roomID)
		require.True(t, client.Rooms[roomID])

		hub.UnsubscribeFromRoom(client, roomID)

		assert.False(t, client.Rooms[roomID])
		_, exists := hub.Rooms[roomID]
		assert.False(t, exists, "empty room should be deleted")
	})

	t.Run("deletes empty room", func(t *testing.T) {
		hub := NewHub(newTestHubLogger())
		client := newTestHubClient(t, hub, "tenant-123", "user-1")

		roomID := "test-room"
		hub.SubscribeToRoom(client, roomID)
		require.NotNil(t, hub.Rooms[roomID])

		hub.UnsubscribeFromRoom(client, roomID)

		_, exists := hub.Rooms[roomID]
		assert.False(t, exists, "empty room should be deleted")
	})

	t.Run("handles non-existent room", func(t *testing.T) {
		hub := NewHub(newTestHubLogger())
		client := newTestHubClient(t, hub, "tenant-123", "user-1")

		assert.NotPanics(t, func() {
			hub.UnsubscribeFromRoom(client, "non-existent-room")
		})
	})
}

func TestHub_ClientCount(t *testing.T) {
	t.Run("returns correct count", func(t *testing.T) {
		hub := NewHub(newTestHubLogger())

		assert.Equal(t, 0, hub.ClientCount())

		client1 := newTestHubClient(t, hub, "tenant-123", "user-1")
		hub.registerClient(client1)
		assert.Equal(t, 1, hub.ClientCount())

		client2 := newTestHubClient(t, hub, "tenant-456", "user-2")
		hub.registerClient(client2)
		assert.Equal(t, 2, hub.ClientCount())

		hub.unregisterClient(client1)
		assert.Equal(t, 1, hub.ClientCount())
	})
}

func TestHub_RoomCount(t *testing.T) {
	t.Run("returns correct count", func(t *testing.T) {
		hub := NewHub(newTestHubLogger())

		assert.Equal(t, 0, hub.RoomCount())

		client1 := newTestHubClient(t, hub, "tenant-123", "user-1")
		hub.registerClient(client1)
		assert.Equal(t, 1, hub.RoomCount())

		client2 := newTestHubClient(t, hub, "tenant-456", "user-2")
		hub.registerClient(client2)
		assert.Equal(t, 2, hub.RoomCount())

		hub.unregisterClient(client1)
		assert.Equal(t, 1, hub.RoomCount())
	})
}

func TestHub_TenantClientCount(t *testing.T) {
	t.Run("returns correct count for tenant", func(t *testing.T) {
		hub := NewHub(newTestHubLogger())

		assert.Equal(t, 0, hub.TenantClientCount("tenant-123"))

		client1 := newTestHubClient(t, hub, "tenant-123", "user-1")
		hub.registerClient(client1)
		assert.Equal(t, 1, hub.TenantClientCount("tenant-123"))

		client2 := newTestHubClient(t, hub, "tenant-123", "user-2")
		hub.registerClient(client2)
		assert.Equal(t, 2, hub.TenantClientCount("tenant-123"))

		client3 := newTestHubClient(t, hub, "tenant-456", "user-3")
		hub.registerClient(client3)
		assert.Equal(t, 2, hub.TenantClientCount("tenant-123"))
		assert.Equal(t, 1, hub.TenantClientCount("tenant-456"))
	})

	t.Run("returns zero for non-existent tenant", func(t *testing.T) {
		hub := NewHub(newTestHubLogger())

		count := hub.TenantClientCount("non-existent")
		assert.Equal(t, 0, count)
	})
}

func TestHub_getOrCreateRoom(t *testing.T) {
	t.Run("creates new room if not exists", func(t *testing.T) {
		hub := NewHub(newTestHubLogger())

		roomID := "new-room"
		room := hub.getOrCreateRoom(roomID)

		assert.NotNil(t, room)
		assert.Equal(t, roomID, room.Name)
		assert.NotNil(t, hub.Rooms[roomID])
	})

	t.Run("returns existing room", func(t *testing.T) {
		hub := NewHub(newTestHubLogger())

		roomID := "existing-room"
		room1 := hub.getOrCreateRoom(roomID)
		room2 := hub.getOrCreateRoom(roomID)

		assert.Equal(t, room1, room2)
		assert.Equal(t, 1, hub.RoomCount())
	})
}
