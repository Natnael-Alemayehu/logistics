package handler

import (
	"net/http"

	"github.com/natnael-alemayehu/logistics/internal/service"
)

type Handler struct {
	Auth     *service.AuthService
	Shipment *service.ShipmentService
	SyncSvc  *service.SyncService
	User     *service.UserService
}

func New(
	auth *service.AuthService,
	shipment *service.ShipmentService,
	sync *service.SyncService,
	user *service.UserService,
) *Handler {
	return &Handler{
		Auth:     auth,
		Shipment: shipment,
		SyncSvc:  sync,
		User:     user,
	}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
