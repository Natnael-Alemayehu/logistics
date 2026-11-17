package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httprate"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/kiflet/internal/db"
	"github.com/natnael-alemayehu/kiflet/internal/model"
	"github.com/natnael-alemayehu/kiflet/internal/service"
)

// The public API (FR-046): a company's own systems booking and following
// its shipments with an API key. Every handler goes through the services
// the dashboard uses, as the key's Actor, so the rules are the same and the
// history says which integration did what. static/openapi.json describes
// it, and a test holds the two together.

// apiRequestsPerMinute is each key's allowance (AC-046.3).
const apiRequestsPerMinute = 300

// maxAPIBody is the most a request may send. A booking is about a
// kilobyte; the driver app's allowance, for photos, is far more.
const maxAPIBody = 64 << 10

type apiRoute struct {
	Method, Path string
	Handler      func(*Server) http.HandlerFunc
}

// publicAPI is every route of the public API, under /api/v1.
var publicAPI = []apiRoute{
	{http.MethodGet, "/shipments", func(s *Server) http.HandlerFunc { return s.apiShipmentList }},
	{http.MethodPost, "/shipments", func(s *Server) http.HandlerFunc { return s.apiShipmentCreate }},
	{http.MethodGet, "/shipments/{ref}", func(s *Server) http.HandlerFunc { return s.apiShipmentGet }},
	{http.MethodPatch, "/shipments/{ref}", func(s *Server) http.HandlerFunc { return s.apiShipmentUpdate }},
	{http.MethodPost, "/shipments/{ref}/assign", func(s *Server) http.HandlerFunc { return s.apiShipmentAssign }},
	{http.MethodPost, "/shipments/{ref}/cancel", func(s *Server) http.HandlerFunc { return s.apiShipmentCancel }},
	{http.MethodGet, "/shipments/{ref}/events", func(s *Server) http.HandlerFunc { return s.apiShipmentEvents }},
	{http.MethodGet, "/drivers", func(s *Server) http.HandlerFunc { return s.apiDriverList }},
}

// mountPublicAPI adds the public API's routes to the /api/v1 router.
func (s *Server) mountPublicAPI(r chi.Router) {
	r.Get("/openapi.json", s.apiSpec)
	perKey := httprate.LimitBy(apiRequestsPerMinute, time.Minute, func(r *http.Request) (string, error) {
		sum := sha256.Sum256([]byte(r.Header.Get("Authorization")))
		return hex.EncodeToString(sum[:8]), nil
	}, httprate.WithLimitHandler(s.tooManyRequests))
	r.Group(func(r chi.Router) {
		r.Use(perKey)
		r.Use(s.requireAPIKey)
		for _, route := range publicAPI {
			r.Method(route.Method, route.Path, route.Handler(s))
		}
	})
}

func (s *Server) apiSpec(w http.ResponseWriter, _ *http.Request) {
	b, err := embedded.ReadFile("static/openapi.json")
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = w.Write(b)
}

type apiActorKey struct{}

// requireAPIKey authenticates a request by its API key.
func (s *Server) requireAPIKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxAPIBody)
		key, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		a, err := s.svc.API.Authenticate(r.Context(), strings.TrimSpace(key), clientInfo(r))
		if err != nil {
			s.apiError(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), apiActorKey{}, a)))
	})
}

func apiActor(r *http.Request) service.Actor {
	a, _ := r.Context().Value(apiActorKey{}).(service.Actor)
	return a
}

// ---------------------------------------------------------------------------
// What the API sends and takes
// ---------------------------------------------------------------------------

type (
	apiPlace    = service.PlaceJSON
	apiParty    = service.PartyJSON
	apiCargo    = service.CargoJSON
	apiShipment = service.ShipmentJSON
	apiEvent    = service.EventJSON
)

func (s *Server) toAPIShipment(sh db.Shipment) apiShipment {
	return service.AsJSON(sh, s.cfg.PublicURL)
}

func timeOf(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	u := t.Time.UTC()
	return &u
}

// optional is a JSON field that tells "absent" from "null" from a value,
// which a partial update needs: absent leaves the field, null clears it.
type optional[T any] struct {
	Set   bool
	Value *T
}

func (o *optional[T]) UnmarshalJSON(b []byte) error {
	o.Set = true
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		o.Value = nil
		return nil
	}
	o.Value = new(T)
	return json.Unmarshal(b, o.Value)
}

// apiShipmentInput is a booking, or a change to one. In a change, a field
// left out stays as it is, and an object that is sent replaces the whole of
// what it names: a destination is sent with its city and address together.
type apiShipmentInput struct {
	Priority          *string             `json:"priority"`
	Origin            *apiPlace           `json:"origin"`
	Destination       *apiPlace           `json:"destination"`
	Customer          *apiParty           `json:"customer"`
	Receiver          optional[apiParty]  `json:"receiver"`
	Cargo             *apiCargo           `json:"cargo"`
	Instructions      *string             `json:"instructions"`
	PlannedPickup     optional[time.Time] `json:"planned_pickup"`
	EstimatedDelivery optional[time.Time] `json:"estimated_delivery"`
	DriverID          string              `json:"driver_id"`
	VehicleID         string              `json:"vehicle_id"`
}

func (in apiShipmentInput) apply(to *service.ShipmentInput) {
	if in.Priority != nil {
		to.Priority = *in.Priority
	}
	if p := in.Origin; p != nil {
		to.OriginCity, to.OriginAddress, to.OriginLat, to.OriginLng = p.City, p.Address, p.Lat, p.Lng
	}
	if p := in.Destination; p != nil {
		to.DestinationCity, to.DestinationAddress, to.DestinationLat, to.DestinationLng = p.City, p.Address, p.Lat, p.Lng
	}
	if c := in.Customer; c != nil {
		to.CustomerName, to.CustomerPhone = c.Name, c.Phone
	}
	if in.Receiver.Set {
		to.ConsigneeName, to.ConsigneePhone = "", ""
		if r := in.Receiver.Value; r != nil {
			to.ConsigneeName, to.ConsigneePhone = r.Name, r.Phone
		}
	}
	if c := in.Cargo; c != nil {
		to.CargoDescription, to.CargoWeightKg, to.CargoValueETB = c.Description, c.WeightKg, c.ValueETB
	}
	if in.Instructions != nil {
		to.SpecialInstructions = *in.Instructions
	}
	if in.PlannedPickup.Set {
		to.PlannedPickup = in.PlannedPickup.Value
	}
	if in.EstimatedDelivery.Set {
		to.EstimatedDelivery = in.EstimatedDelivery.Value
	}
}

// decodeAPI reads a request body, refusing fields the API doesn't have: a
// misspelt field should be an error, not a booking without it.
func decodeAPI(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		msg := "The request body is not valid JSON."
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			msg = "The request body is larger than the API takes (64 KB)."
		} else if f, ok := strings.CutPrefix(err.Error(), "json: unknown field "); ok {
			msg = "The request has a field the API doesn't know: " + f + "."
		}
		return &service.Error{Kind: service.KindInvalid, Message: msg}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Shipments
// ---------------------------------------------------------------------------

// apiFind loads the shipment a path names, by tracking number or by ID.
func (s *Server) apiFind(r *http.Request) (db.Shipment, error) {
	ref := chi.URLParam(r, "ref")
	if id, err := parseID(ref); err == nil {
		return s.svc.Shipments.Get(r.Context(), apiActor(r).TenantID, id)
	}
	return s.svc.Shipments.ByNumber(r.Context(), apiActor(r).TenantID, ref)
}

func (s *Server) apiShipmentList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	perPage, _ := strconv.Atoi(q.Get("per_page"))
	if perPage <= 0 || perPage > 100 {
		perPage = 50
	}
	if page < 1 {
		page = 1
	}
	f := service.ShipmentFilter{
		DriverID: q.Get("driver_id"), Priority: q.Get("priority"), Query: q.Get("q"), Sort: q.Get("sort"), Page: page, PerPage: perPage,
	}
	if st := q.Get("status"); st != "" {
		for _, one := range strings.Split(st, ",") {
			if !model.IsValidStatus(one) {
				s.apiError(w, r, &service.Error{Kind: service.KindInvalid, Field: "status", Message: "Unknown status " + strconv.Quote(one) + "."})
				return
			}
			f.Statuses = append(f.Statuses, one)
		}
	}
	for name, to := range map[string]**time.Time{"created_from": &f.From, "created_to": &f.To} {
		v := q.Get(name)
		if v == "" {
			continue
		}
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			s.apiError(w, r, &service.Error{Kind: service.KindInvalid, Field: name, Message: "Write " + name + " as 2026-11-20T08:00:00+03:00."})
			return
		}
		*to = &t
	}
	rows, total, err := s.svc.Shipments.List(r.Context(), apiActor(r).TenantID, f)
	if err != nil {
		s.apiError(w, r, err)
		return
	}
	data := make([]apiShipment, 0, len(rows))
	for _, row := range rows {
		data = append(data, s.toAPIShipment(row.Shipment))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "page": page, "per_page": perPage, "total": total})
}

func (s *Server) apiShipmentCreate(w http.ResponseWriter, r *http.Request) {
	var in apiShipmentInput
	if err := decodeAPI(r, &in); err != nil {
		s.apiError(w, r, err)
		return
	}
	booking := service.ShipmentInput{DriverID: in.DriverID, VehicleID: in.VehicleID}
	in.apply(&booking)
	sh, err := s.svc.Shipments.Create(r.Context(), apiActor(r), booking)
	if err != nil {
		s.apiError(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/v1/shipments/"+sh.TrackingNumber)
	writeJSON(w, http.StatusCreated, s.toAPIShipment(sh))
}

func (s *Server) apiShipmentGet(w http.ResponseWriter, r *http.Request) {
	sh, err := s.apiFind(r)
	if err != nil {
		s.apiError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, s.toAPIShipment(sh))
}

func (s *Server) apiShipmentUpdate(w http.ResponseWriter, r *http.Request) {
	var in apiShipmentInput
	if err := decodeAPI(r, &in); err != nil {
		s.apiError(w, r, err)
		return
	}
	if in.DriverID != "" || in.VehicleID != "" {
		s.apiError(w, r, &service.Error{Kind: service.KindInvalid, Field: "driver_id", Message: "Change the driver or vehicle with POST /shipments/{ref}/assign."})
		return
	}
	sh, err := s.apiFind(r)
	if err != nil {
		s.apiError(w, r, err)
		return
	}
	change := service.InputOf(sh)
	in.apply(&change)
	out, err := s.svc.Shipments.Update(r.Context(), apiActor(r), sh.ID, change)
	if err != nil {
		s.apiError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, s.toAPIShipment(out))
}

func (s *Server) apiShipmentAssign(w http.ResponseWriter, r *http.Request) {
	var in struct {
		DriverID  *string `json:"driver_id"`
		VehicleID string  `json:"vehicle_id"`
	}
	if err := decodeAPI(r, &in); err != nil {
		s.apiError(w, r, err)
		return
	}
	sh, err := s.apiFind(r)
	if err != nil {
		s.apiError(w, r, err)
		return
	}
	var out db.Shipment
	if in.DriverID == nil || *in.DriverID == "" {
		out, err = s.svc.Shipments.Unassign(r.Context(), apiActor(r), sh.ID)
	} else {
		out, err = s.svc.Shipments.Assign(r.Context(), apiActor(r), sh.ID, *in.DriverID, in.VehicleID)
	}
	if err != nil {
		s.apiError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, s.toAPIShipment(out))
}

func (s *Server) apiShipmentCancel(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Reason string `json:"reason"`
		Note   string `json:"note"`
	}
	if err := decodeAPI(r, &in); err != nil {
		s.apiError(w, r, err)
		return
	}
	sh, err := s.apiFind(r)
	if err != nil {
		s.apiError(w, r, err)
		return
	}
	out, err := s.svc.Shipments.Cancel(r.Context(), apiActor(r), sh.ID, in.Reason, in.Note)
	if err != nil {
		s.apiError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, s.toAPIShipment(out))
}

type apiPosition struct {
	Lat        float64   `json:"lat"`
	Lng        float64   `json:"lng"`
	RecordedAt time.Time `json:"recorded_at"`
}

// apiShipmentEvents is a shipment's history, oldest first, and where its
// truck was last seen.
func (s *Server) apiShipmentEvents(w http.ResponseWriter, r *http.Request) {
	sh, err := s.apiFind(r)
	if err != nil {
		s.apiError(w, r, err)
		return
	}
	d, err := s.svc.Shipments.Detail(r.Context(), sh.TenantID, sh.ID)
	if err != nil {
		s.apiError(w, r, err)
		return
	}
	events := make([]apiEvent, 0, len(d.Events))
	for _, e := range d.Events {
		events = append(events, apiEvent{
			Type: e.EventType, FromStatus: deref(e.FromStatus), ToStatus: deref(e.ToStatus), Reason: deref(e.Reason), Note: deref(e.Note),
			Source: e.Source, Actor: e.ActorName, Lat: e.Latitude, Lng: e.Longitude, At: e.OccurredAt.Time.UTC(),
		})
	}
	// Detail lists the newest first; an integration reads a history forwards.
	if len(events) > 1 && events[0].At.After(events[len(events)-1].At) {
		for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
			events[i], events[j] = events[j], events[i]
		}
	}
	var last *apiPosition
	if n := len(d.Route); n > 0 {
		p := d.Route[n-1]
		last = &apiPosition{Lat: p.Latitude, Lng: p.Longitude, RecordedAt: p.RecordedAt.Time.UTC()}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": events, "last_position": last})
}

// ---------------------------------------------------------------------------
// Drivers
// ---------------------------------------------------------------------------

type apiDriver struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Phone           string     `json:"phone"`
	Active          bool       `json:"active"`
	VehiclePlate    string     `json:"vehicle_plate"`
	ActiveShipments int64      `json:"active_shipments"`
	LastSyncAt      *time.Time `json:"last_sync_at"`
}

func (s *Server) apiDriverList(w http.ResponseWriter, r *http.Request) {
	rows, err := s.svc.Users.Drivers(r.Context(), apiActor(r).TenantID, r.URL.Query().Get("active") != "false", r.URL.Query().Get("q"))
	if err != nil {
		s.apiError(w, r, err)
		return
	}
	data := make([]apiDriver, 0, len(rows))
	for _, d := range rows {
		data = append(data, apiDriver{
			ID: d.ID.String(), Name: d.FullName, Phone: deref(d.Phone), Active: d.IsActive, VehiclePlate: deref(d.VehiclePlate),
			ActiveShipments: d.ActiveShipments, LastSyncAt: timeOf(d.LastSyncAt),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}
