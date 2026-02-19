package websocket

import (
	"io"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newClientTestLogger() zerolog.Logger {
	return zerolog.New(io.Discard)
}

func TestNewClient(t *testing.T) {
	t.Run("creates client with correct fields", func(t *testing.T) {
		hub := NewHub(newClientTestLogger())

		client := NewClient(hub, nil, "tenant-123", "user-456", "driver")

		require.NotNil(t, client)
		assert.Equal(t, hub, client.Hub)
		assert.Nil(t, client.Conn)
		assert.Equal(t, "tenant-123", client.TenantID)
		assert.Equal(t, "user-456", client.UserID)
		assert.Equal(t, "driver", client.Role)
		assert.NotNil(t, client.Send)
		assert.NotNil(t, client.Rooms)
		assert.Empty(t, client.Rooms)
	})

	t.Run("creates client with different roles", func(t *testing.T) {
		hub := NewHub(newClientTestLogger())

		driverClient := NewClient(hub, nil, "tenant-123", "user-1", "driver")
		adminClient := NewClient(hub, nil, "tenant-123", "user-2", "admin")
		dispatcherClient := NewClient(hub, nil, "tenant-123", "user-3", "dispatcher")

		assert.Equal(t, "driver", driverClient.Role)
		assert.Equal(t, "admin", adminClient.Role)
		assert.Equal(t, "dispatcher", dispatcherClient.Role)
	})

	t.Run("initializes send channel with buffer", func(t *testing.T) {
		hub := NewHub(newClientTestLogger())
		client := NewClient(hub, nil, "tenant-123", "user-456", "driver")

		for i := 0; i < 256; i++ {
			select {
			case client.Send <- []byte("test"):
			default:
				t.Fatalf("send channel should have buffer capacity of 256, failed at %d", i)
			}
		}
	})
}

func TestClient_IsConnected(t *testing.T) {
	t.Run("returns true for new client", func(t *testing.T) {
		hub := NewHub(newClientTestLogger())
		client := NewClient(hub, nil, "tenant-123", "user-456", "driver")

		assert.True(t, client.IsConnected())
	})

	t.Run("returns false after disconnect", func(t *testing.T) {
		hub := NewHub(newClientTestLogger())
		client := NewClient(hub, nil, "tenant-123", "user-456", "driver")

		require.True(t, client.IsConnected())
		client.Disconnect()

		assert.False(t, client.IsConnected())
	})

	t.Run("returns correct state after multiple disconnect calls", func(t *testing.T) {
		hub := NewHub(newClientTestLogger())
		client := NewClient(hub, nil, "tenant-123", "user-456", "driver")

		client.Disconnect()
		assert.False(t, client.IsConnected())

		client.Disconnect()
		assert.False(t, client.IsConnected())
	})
}

func TestClient_Disconnect(t *testing.T) {
	t.Run("cancels context", func(t *testing.T) {
		hub := NewHub(newClientTestLogger())
		client := NewClient(hub, nil, "tenant-123", "user-456", "driver")

		assert.True(t, client.IsConnected())

		client.Disconnect()

		assert.False(t, client.IsConnected())
	})

	t.Run("is idempotent", func(t *testing.T) {
		hub := NewHub(newClientTestLogger())
		client := NewClient(hub, nil, "tenant-123", "user-456", "driver")

		assert.NotPanics(t, func() {
			client.Disconnect()
			client.Disconnect()
			client.Disconnect()
		})
	})
}

func TestClient_Send(t *testing.T) {
	t.Run("channel works correctly", func(t *testing.T) {
		hub := NewHub(newClientTestLogger())
		client := NewClient(hub, nil, "tenant-123", "user-456", "driver")

		message := []byte(`{"type":"test"}`)

		select {
		case client.Send <- message:
		default:
			t.Fatal("should be able to send to buffer")
		}

		received := <-client.Send
		assert.Equal(t, message, received)
	})

	t.Run("channel handles multiple messages", func(t *testing.T) {
		hub := NewHub(newClientTestLogger())
		client := NewClient(hub, nil, "tenant-123", "user-456", "driver")

		messages := [][]byte{
			[]byte(`{"type":"msg1"}`),
			[]byte(`{"type":"msg2"}`),
			[]byte(`{"type":"msg3"}`),
		}

		for _, msg := range messages {
			client.Send <- msg
		}

		for _, expected := range messages {
			received := <-client.Send
			assert.Equal(t, expected, received)
		}
	})
}

func TestClient_Rooms(t *testing.T) {
	t.Run("rooms map is initialized empty", func(t *testing.T) {
		hub := NewHub(newClientTestLogger())
		client := NewClient(hub, nil, "tenant-123", "user-456", "driver")

		assert.NotNil(t, client.Rooms)
		assert.Empty(t, client.Rooms)
	})

	t.Run("can track room subscriptions", func(t *testing.T) {
		hub := NewHub(newClientTestLogger())
		client := NewClient(hub, nil, "tenant-123", "user-456", "driver")

		room1 := "room-1"
		room2 := "room-2"

		client.Rooms[room1] = true
		client.Rooms[room2] = true

		assert.True(t, client.Rooms[room1])
		assert.True(t, client.Rooms[room2])
		assert.Equal(t, 2, len(client.Rooms))
	})
}
