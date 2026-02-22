package handler

import (
	"net/http"
	"strings"
	"time"

	gorillaws "github.com/gorilla/websocket"
	"github.com/natnael-alemayehu/logistics/pkg/jwt"
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
	Hub        *websocket.Hub
	JWTManager *jwt.JWTManager
}

func NewWSHandler(hub *websocket.Hub, jwtManager *jwt.JWTManager) *WSHandler {
	return &WSHandler{
		Hub:        hub,
		JWTManager: jwtManager,
	}
}

func (h *WSHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	token := h.extractToken(r)
	if token == "" {
		http.Error(w, "Missing authentication token", http.StatusUnauthorized)
		return
	}

	claims, err := h.JWTManager.Validate(token)
	if err != nil {
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
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

func (h *WSHandler) extractToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
			return parts[1]
		}
	}

	token := r.URL.Query().Get("token")
	if token != "" {
		return token
	}

	return ""
}
