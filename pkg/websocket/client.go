// Package websocket keeps the dashboards' live feed: a Client is one
// browser's connection, and the Hub sends a company's updates to that
// company's clients.
package websocket

import (
	"context"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 30 * time.Second
	maxMessageSize = 512
)

type Client struct {
	Hub      *Hub
	Conn     *websocket.Conn
	Send     chan []byte
	TenantID string
	UserID   string
	Role     string
	ctx      context.Context
	cancel   context.CancelFunc
}

func NewClient(hub *Hub, conn *websocket.Conn, tenantID, userID, role string) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		Hub:      hub,
		Conn:     conn,
		Send:     make(chan []byte, 256),
		TenantID: tenantID,
		UserID:   userID,
		Role:     role,
		ctx:      ctx,
		cancel:   cancel,
	}
}

func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	if err := c.Conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		return
	}
	c.Conn.SetPongHandler(func(string) error {
		return c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			_, _, err := c.Conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
		c.cancel()
	}()

	for {
		select {
		case <-c.ctx.Done():
			c.goodbye()
			return
		case message, ok := <-c.Send:
			if !ok {
				c.goodbye()
				return
			}
			if err := c.Conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				return
			}

			// One JSON document per frame: the dashboard parses each frame
			// on its own, so messages must never be joined.
			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.Conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				return
			}
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// goodbye tells the browser the connection is closing. It is a courtesy:
// the connection closes whether it arrives or not.
func (c *Client) goodbye() {
	_ = c.Conn.WriteControl(websocket.CloseMessage, nil, time.Now().Add(writeWait))
}

func (c *Client) Disconnect() {
	c.cancel()
}

// Done is closed once the client has disconnected.
func (c *Client) Done() <-chan struct{} { return c.ctx.Done() }
