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
	return NewClient(hub, nil, tenantID, userID, "dispatcher")
}

func TestNewHub(t *testing.T) {
	hub := NewHub(newTestHubLogger())

	require.NotNil(t, hub)
	assert.NotNil(t, hub.Register)
	assert.NotNil(t, hub.Unregister)
	assert.Equal(t, 0, hub.ClientCount())
}

func TestHub_registerClient(t *testing.T) {
	t.Run("adds the client to its company", func(t *testing.T) {
		hub := NewHub(newTestHubLogger())
		client := newTestHubClient(t, hub, "tenant-123", "user-1")

		hub.registerClient(client)

		assert.True(t, hub.tenants["tenant-123"][client])
		assert.Equal(t, 1, hub.ClientCount())
	})

	t.Run("keeps a company's clients together", func(t *testing.T) {
		hub := NewHub(newTestHubLogger())
		client1 := newTestHubClient(t, hub, "tenant-123", "user-1")
		client2 := newTestHubClient(t, hub, "tenant-123", "user-2")

		hub.registerClient(client1)
		hub.registerClient(client2)

		assert.Equal(t, 2, hub.ClientCount())
		assert.Len(t, hub.tenants, 1)
		assert.Len(t, hub.tenants["tenant-123"], 2)
	})

	t.Run("keeps companies apart", func(t *testing.T) {
		hub := NewHub(newTestHubLogger())
		client1 := newTestHubClient(t, hub, "tenant-123", "user-1")
		client2 := newTestHubClient(t, hub, "tenant-456", "user-2")

		hub.registerClient(client1)
		hub.registerClient(client2)

		assert.Equal(t, 2, hub.ClientCount())
		assert.Len(t, hub.tenants["tenant-123"], 1)
		assert.Len(t, hub.tenants["tenant-456"], 1)
	})

	t.Run("counts a client registered twice once", func(t *testing.T) {
		hub := NewHub(newTestHubLogger())
		client := newTestHubClient(t, hub, "tenant-123", "user-1")

		hub.registerClient(client)
		hub.registerClient(client)

		assert.Equal(t, 1, hub.ClientCount())
	})
}

func TestHub_unregisterClient(t *testing.T) {
	t.Run("removes the client and closes its channel", func(t *testing.T) {
		hub := NewHub(newTestHubLogger())
		client := newTestHubClient(t, hub, "tenant-123", "user-1")
		hub.registerClient(client)
		require.Equal(t, 1, hub.ClientCount())

		hub.unregisterClient(client)

		assert.Equal(t, 0, hub.ClientCount())
		_, open := <-client.Send
		assert.False(t, open, "the client's channel should be closed")
		assert.Empty(t, hub.tenants, "a company with no clients should be forgotten")
	})

	t.Run("ignores a client it doesn't know", func(t *testing.T) {
		hub := NewHub(newTestHubLogger())
		client := newTestHubClient(t, hub, "tenant-123", "user-1")

		assert.NotPanics(t, func() {
			hub.unregisterClient(client)
		})
	})

	t.Run("ignores a client unregistered twice", func(t *testing.T) {
		hub := NewHub(newTestHubLogger())
		client := newTestHubClient(t, hub, "tenant-123", "user-1")
		hub.registerClient(client)

		assert.NotPanics(t, func() {
			hub.unregisterClient(client)
			hub.unregisterClient(client)
		})
	})

	t.Run("keeps the company's other clients", func(t *testing.T) {
		hub := NewHub(newTestHubLogger())
		client1 := newTestHubClient(t, hub, "tenant-123", "user-1")
		client2 := newTestHubClient(t, hub, "tenant-123", "user-2")

		hub.registerClient(client1)
		hub.registerClient(client2)
		require.Equal(t, 2, hub.ClientCount())

		hub.unregisterClient(client1)

		assert.Equal(t, 1, hub.ClientCount())
		assert.True(t, hub.tenants["tenant-123"][client2])
	})
}

func TestHub_BroadcastToTenant(t *testing.T) {
	t.Run("reaches the company's clients and nobody else's", func(t *testing.T) {
		hub := NewHub(newTestHubLogger())
		go hub.Run()
		client1 := newTestHubClient(t, hub, "tenant-123", "user-1")
		client2 := newTestHubClient(t, hub, "tenant-123", "user-2")
		other := newTestHubClient(t, hub, "tenant-456", "user-3")
		hub.Register <- client1
		hub.Register <- client2
		hub.Register <- other

		message := []byte(`{"type":"test"}`)
		hub.BroadcastToTenant("tenant-123", message)

		for _, client := range []*Client{client1, client2} {
			select {
			case msg := <-client.Send:
				assert.Equal(t, message, msg)
			case <-time.After(2 * time.Second):
				t.Fatal("a client of the company did not receive the message")
			}
		}
		select {
		case <-other.Send:
			t.Fatal("another company's client received the message")
		case <-time.After(100 * time.Millisecond):
		}
	})

	t.Run("does nothing for a company with no clients", func(t *testing.T) {
		hub := NewHub(newTestHubLogger())
		client := newTestHubClient(t, hub, "tenant-123", "user-1")
		hub.registerClient(client)

		hub.send(tenantMessage{tenantID: "tenant-456", message: []byte("x")})

		assert.Empty(t, client.Send)
	})

	t.Run("skips a client whose buffer is full", func(t *testing.T) {
		hub := NewHub(newTestHubLogger())
		slow := newTestHubClient(t, hub, "tenant-123", "user-1")
		quick := newTestHubClient(t, hub, "tenant-123", "user-2")
		hub.registerClient(slow)
		hub.registerClient(quick)
		for len(slow.Send) < cap(slow.Send) {
			slow.Send <- []byte("earlier")
		}

		message := []byte(`{"type":"test"}`)
		hub.send(tenantMessage{tenantID: "tenant-123", message: message})

		assert.Len(t, slow.Send, cap(slow.Send))
		assert.Equal(t, message, <-quick.Send)
	})
}

func TestHub_ClientCount(t *testing.T) {
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
}
