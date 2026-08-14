package handler

import (
	"net/http"
	"time"

	gorillaws "github.com/gorilla/websocket"
	"github.com/natnael-alemayehu/logistics/internal/middleware"
	"github.com/natnael-alemayehu/logistics/pkg/websocket"
)

const (
	wsReadBufferSize  = 1024
	wsWriteBufferSize = 1024
)

var upgrader = gorillaws.Upgrader{
	ReadBufferSize:    wsReadBufferSize,
	WriteBufferSize:   wsWriteBufferSize,
	HandshakeTimeout:  10 * time.Second,
	EnableCompression: true,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type WSHandler struct {
	Hub *websocket.Hub
}

func NewWSHandler(hub *websocket.Hub) *WSHandler {
	return &WSHandler{
		Hub: hub,
	}
}

func (h *WSHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := websocket.NewClient(h.Hub, conn, claims.TenantID, claims.UserID, claims.Role)
	client.Rooms[websocket.GetTenantRoomID(claims.TenantID)] = true

	h.Hub.Register <- client

	go client.WritePump()
	go client.ReadPump()
}
