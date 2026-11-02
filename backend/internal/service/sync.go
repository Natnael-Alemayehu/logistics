package service

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/logistics/internal/db"
	"github.com/natnael-alemayehu/logistics/internal/model"
)

// SyncService is the driver app's offline sync (FR-009–013): the device
// pushes what it recorded while offline and pulls its current work in one
// round trip.
type SyncService struct {
	*core
	shipments *ShipmentService
	media     *MediaService
}

// SyncRequest is one sync round trip from the driver app.
type SyncRequest struct {
	DeviceID       string     `json:"device_id"`
	AppVersion     string     `json:"app_version"`
	DeviceTime     *time.Time `json:"device_time"`
	Battery        *int       `json:"battery"`
	Pending        *int       `json:"pending"`
	StorageKB      *int       `json:"storage_kb"`
	TrackingPaused bool       `json:"tracking_paused"`
	// Since is the cursor from the previous response; empty on first sync.
	Since *time.Time `json:"since"`

	Statuses  []SyncStatus   `json:"statuses"`
	PODs      []SyncPOD      `json:"pods"`
	Locations []SyncLocation `json:"locations"`
	Messages  []SyncMessage  `json:"messages"`
}

// SyncStatus is a status change made on the device.
type SyncStatus struct {
	ClientID   string    `json:"client_id"`
	ShipmentID string    `json:"shipment_id"`
	Status     string    `json:"status"`
	Reason     string    `json:"reason"`
	Note       string    `json:"note"`
	Lat        *float64  `json:"lat"`
	Lng        *float64  `json:"lng"`
	RecordedAt time.Time `json:"recorded_at"`
}

// SyncPOD is a proof of delivery captured on the device. Images are data
// URLs.
type SyncPOD struct {
	ClientID       string    `json:"client_id"`
	ShipmentID     string    `json:"shipment_id"`
	RecipientName  string    `json:"recipient_name"`
	RecipientPhone string    `json:"recipient_phone"`
	Signature      string    `json:"signature"`
	Photos         []string  `json:"photos"`
	Notes          string    `json:"notes"`
	Lat            *float64  `json:"lat"`
	Lng            *float64  `json:"lng"`
	RecordedAt     time.Time `json:"recorded_at"`
}

// SyncLocation is a GPS fix.
type SyncLocation struct {
	ClientID   string    `json:"client_id"`
	ShipmentID string    `json:"shipment_id"`
	Lat        float64   `json:"lat"`
	Lng        float64   `json:"lng"`
	Accuracy   *float64  `json:"accuracy"`
	Speed      *float64  `json:"speed"`
	Heading    *float64  `json:"heading"`
	Battery    *int      `json:"battery"`
	RecordedAt time.Time `json:"recorded_at"`
}

// SyncMessage is a note from the driver to the office.
type SyncMessage struct {
	ClientID   string    `json:"client_id"`
	ShipmentID string    `json:"shipment_id"`
	Text       string    `json:"text"`
	RecordedAt time.Time `json:"recorded_at"`
}

// SyncResult tells the device what became of one queued item. OK and
// rejected items leave the queue; Retry ones stay for the next sync.
type SyncResult struct {
	ClientID string `json:"client_id"`
	Kind     string `json:"kind"`
	OK       bool   `json:"ok"`
	Retry    bool   `json:"retry,omitempty"`
	Code     string `json:"code,omitempty"`
	Message  string `json:"message,omitempty"`
}

// SyncResponse is the server's half of the round trip.
type SyncResponse struct {
	ServerTime time.Time `json:"server_time"`
	// Cursor is passed back as Since next time.
	Cursor             time.Time        `json:"cursor"`
	Results            []SyncResult     `json:"results"`
	LocationsStored    int              `json:"locations_stored"`
	Shipments          []DriverShipment `json:"shipments"`
	RemovedShipmentIDs []string         `json:"removed_shipment_ids"`
	Messages           []DriverMessage  `json:"messages"`
	Driver             DriverProfile    `json:"driver"`
}

// DriverShipment is a shipment as the driver app holds it.
type DriverShipment struct {
	ID                 string     `json:"id"`
	TrackingNumber     string     `json:"tracking_number"`
	Ref                string     `json:"ref"`
	Status             string     `json:"status"`
	StatusReason       string     `json:"status_reason,omitempty"`
	Priority           string     `json:"priority"`
	OriginCity         string     `json:"origin_city"`
	OriginAddress      string     `json:"origin_address"`
	OriginLat          *float64   `json:"origin_lat"`
	OriginLng          *float64   `json:"origin_lng"`
	DestinationCity    string     `json:"destination_city"`
	DestinationAddress string     `json:"destination_address"`
	DestinationLat     *float64   `json:"destination_lat"`
	DestinationLng     *float64   `json:"destination_lng"`
	DestinationPrecise bool       `json:"destination_precise"`
	CustomerName       string     `json:"customer_name"`
	CustomerPhone      string     `json:"customer_phone"`
	Cargo              string     `json:"cargo"`
	WeightKg           float64    `json:"weight_kg"`
	Instructions       string     `json:"instructions"`
	Vehicle            string     `json:"vehicle"`
	ETA                *time.Time `json:"eta"`
	PlannedPickup      *time.Time `json:"planned_pickup"`
	PickedUpAt         *time.Time `json:"picked_up_at"`
	DeliveredAt        *time.Time `json:"delivered_at"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// DriverMessage is a note from the office, shown in the app.
type DriverMessage struct {
	ID             string    `json:"id"`
	ShipmentID     string    `json:"shipment_id"`
	TrackingNumber string    `json:"tracking_number"`
	Text           string    `json:"text"`
	From           string    `json:"from"`
	At             time.Time `json:"at"`
}

// DriverProfile is who the app is signed in as.
type DriverProfile struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Phone         string `json:"phone"`
	Company       string `json:"company"`
	Vehicle       string `json:"vehicle"`
	MustChangePIN bool   `json:"must_change_pin"`
	SMSNumber     string `json:"sms_number"`
}

// Sync limits per request, beyond which the device sends the rest next time.
const (
	maxSyncStatuses  = 200
	maxSyncPODs      = 20
	maxSyncLocations = 5000
	maxSyncMessages  = 100
	// Device clocks within two minutes of the server's are left alone.
	clockTolerance = 2 * time.Minute
)

// Sync processes a round trip.
func (s *SyncService) Sync(ctx context.Context, p *Principal, req SyncRequest, ci ClientInfo) (*SyncResponse, error) {
	if !p.IsDriver() {
		return nil, ErrNotDriver
	}
	now := s.now()
	actor := p.Actor(SourceApp, ci.IP, ci.UserAgent)

	// Phones are often set to the wrong time, and everything the device
	// recorded is stamped with its clock. The difference between its clock
	// and ours at the moment of the request corrects every timestamp in it.
	var offset time.Duration
	if req.DeviceTime != nil {
		if d := now.Sub(*req.DeviceTime); d > clockTolerance || d < -clockTolerance {
			offset = d
		}
	}
	correct := func(t time.Time) time.Time {
		if t.IsZero() {
			return now
		}
		t = t.Add(offset)
		if t.After(now) {
			return now
		}
		return t
	}

	resp := &SyncResponse{ServerTime: now, Results: []SyncResult{}, Shipments: []DriverShipment{},
		RemovedShipmentIDs: []string{}, Messages: []DriverMessage{}}

	// Status changes and deliveries are applied in the order they happened,
	// so "arrived" lands before the POD that completes the shipment.
	type op struct {
		at     time.Time
		status *SyncStatus
		pod    *SyncPOD
	}
	var ops []op
	for i := range capSlice(req.Statuses, maxSyncStatuses) {
		ops = append(ops, op{at: correct(req.Statuses[i].RecordedAt), status: &req.Statuses[i]})
	}
	for i := range capSlice(req.PODs, maxSyncPODs) {
		ops = append(ops, op{at: correct(req.PODs[i].RecordedAt), pod: &req.PODs[i]})
	}
	sort.SliceStable(ops, func(i, j int) bool { return ops[i].at.Before(ops[j].at) })

	for _, o := range ops {
		if o.status != nil {
			resp.Results = append(resp.Results, s.applyStatus(ctx, actor, *o.status, o.at))
		} else {
			resp.Results = append(resp.Results, s.applyPOD(ctx, actor, *o.pod, o.at))
		}
	}

	for _, m := range capSlice(req.Messages, maxSyncMessages) {
		r := SyncResult{ClientID: m.ClientID, Kind: "message", OK: true}
		id, err := parseUUID(m.ShipmentID)
		if err == nil {
			_, err = s.shipments.Note(ctx, actor, id, m.Text, false, false, m.ClientID, correct(m.RecordedAt))
		} else {
			err = notFound("Shipment not found.")
		}
		resp.Results = append(resp.Results, resultFor(r, err))
	}

	// Positions may only be attributed to the driver's own shipments.
	mine, err := s.q.ListShipmentsForDriverSync(ctx, db.ListShipmentsForDriverSyncParams{DriverID: p.UserID, TenantID: p.TenantID})
	if err != nil {
		return nil, err
	}
	allowed := make(map[pgtype.UUID]bool, len(mine))
	for _, sh := range mine {
		allowed[sh.ID] = true
	}
	if locs := capSlice(req.Locations, maxSyncLocations); len(locs) > 0 {
		pings := make([]Ping, 0, len(locs))
		for _, l := range locs {
			pings = append(pings, Ping{
				ClientID: l.ClientID, ShipmentID: toUUID(l.ShipmentID), Lat: l.Lat, Lng: l.Lng,
				Accuracy: l.Accuracy, Speed: l.Speed, Heading: l.Heading, Battery: l.Battery,
				RecordedAt: correct(l.RecordedAt),
			})
		}
		tracking := &TrackingService{core: s.core}
		resp.LocationsStored, _ = tracking.record(ctx, actor, req.DeviceID, pings, allowed)
	}

	var battery, pending, storageKB, clockOffset *int32
	if req.Battery != nil && *req.Battery >= 0 && *req.Battery <= 100 {
		battery = ptr(int32(*req.Battery))
	}
	if req.Pending != nil {
		pending = ptr(int32(*req.Pending))
	}
	if req.StorageKB != nil {
		storageKB = ptr(int32(*req.StorageKB))
	}
	if req.DeviceTime != nil {
		clockOffset = ptr(int32(now.Sub(*req.DeviceTime).Seconds()))
	}
	if err := s.q.UpsertSyncMetadata(ctx, db.UpsertSyncMetadataParams{
		DriverID: p.UserID, DeviceID: toText(truncate(req.DeviceID, 100)), AppVersion: toText(truncate(req.AppVersion, 40)),
		BatteryLevel: battery, PendingSyncCount: pending, StorageUsedKb: storageKB,
		ClockOffsetSeconds: clockOffset, TrackingPaused: req.TrackingPaused,
	}); err != nil {
		s.log.Error().Err(err).Msg("upsert sync metadata")
	}
	_ = s.q.AcknowledgeDriverAlerts(ctx, db.AcknowledgeDriverAlertsParams{DriverID: p.UserID, AlertType: AlertDriverOffline})

	if err := s.pull(ctx, p, req.Since, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func capSlice[T any](s []T, n int) []T {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func (s *SyncService) applyStatus(ctx context.Context, a Actor, st SyncStatus, at time.Time) SyncResult {
	r := SyncResult{ClientID: st.ClientID, Kind: "status", OK: true}
	id, err := parseUUID(st.ShipmentID)
	if err != nil {
		return resultFor(r, notFound("Shipment not found."))
	}
	_, err = s.shipments.ChangeStatus(ctx, a, id, StatusChange{
		Status: st.Status, Reason: st.Reason, Note: st.Note, Lat: st.Lat, Lng: st.Lng,
		OccurredAt: at, By: model.ActorDriver, ClientID: st.ClientID,
	})
	if errors.Is(err, model.ErrSameStatus) {
		err = nil
	}
	return resultFor(r, err)
}

func (s *SyncService) applyPOD(ctx context.Context, a Actor, in SyncPOD, at time.Time) SyncResult {
	r := SyncResult{ClientID: in.ClientID, Kind: "pod", OK: true}
	id, err := parseUUID(in.ShipmentID)
	if err != nil {
		return resultFor(r, notFound("Shipment not found."))
	}
	pod := PODInput{
		RecipientName: in.RecipientName, RecipientPhone: in.RecipientPhone, Notes: in.Notes,
		Lat: in.Lat, Lng: in.Lng, RecordedAt: at, ClientID: in.ClientID,
	}
	// Whatever will be refused is refused before any image is stored.
	if err := pod.Check(); err != nil {
		return resultFor(r, err)
	}
	// A replay of a proof already stored must not upload its images again.
	if in.ClientID != "" {
		prior, err := s.q.GetPODByClientID(ctx, db.GetPODByClientIDParams{TenantID: a.TenantID, DriverID: a.UserID, ClientID: &in.ClientID})
		if err == nil {
			if prior.Status != PODAccepted {
				return resultFor(r, conflict(heldMessage(deref(prior.HeldReason))))
			}
			return r
		}
	}
	sh, err := s.shipments.Get(ctx, a.TenantID, id)
	if err != nil {
		return resultFor(r, err)
	}
	if sh.DriverID != a.UserID {
		// Handed over while this driver was offline: their proof is held for
		// review, but only if the shipment was ever theirs.
		had, err := s.q.WasAssignedTo(ctx, db.WasAssignedToParams{TenantID: a.TenantID, ShipmentID: id, DriverID: a.UserID})
		if err != nil {
			return resultFor(r, err)
		}
		if !had {
			return resultFor(r, ErrNotAssigned)
		}
	}
	sig, photos, err := s.media.SavePOD(ctx, id.String(), in.Signature, in.Photos)
	if err != nil {
		return resultFor(r, err)
	}
	pod.SignatureKey, pod.PhotoKeys = sig, photos
	_, err = s.shipments.RecordPOD(ctx, a, id, pod)
	return resultFor(r, err)
}

// resultFor classifies an outcome: domain errors are final, anything else is
// a fault on our side the device should retry.
func resultFor(r SyncResult, err error) SyncResult {
	if err == nil {
		return r
	}
	r.OK = false
	if errors.Is(err, ErrStaleUpdate) {
		r.Code, r.Message = "STALE", ErrStaleUpdate.Message
		return r
	}
	if de, ok := AsError(err); ok {
		r.Code, r.Message = de.Kind.Code(), de.Message
		return r
	}
	r.Retry = true
	r.Code, r.Message = "ERROR", "The server couldn't save this. It will be retried."
	return r
}

// pull fills in what the device should now hold.
func (s *SyncService) pull(ctx context.Context, p *Principal, since *time.Time, resp *SyncResponse) error {
	// The cursor trails the clock slightly so that a write committing as this
	// read runs is picked up next time; the device upserts, so seeing a
	// change twice is harmless.
	resp.Cursor = resp.ServerTime.Add(-5 * time.Second)

	var sinceTS pgtype.Timestamptz
	if since != nil {
		sinceTS = toTimestamp(*since)
	}
	ships, err := s.q.ListShipmentsForDriverSync(ctx, db.ListShipmentsForDriverSyncParams{
		DriverID: p.UserID, TenantID: p.TenantID, Since: sinceTS,
	})
	if err != nil {
		return err
	}
	plates := map[pgtype.UUID]string{}
	if vs, err := s.q.ListActiveVehiclesByTenant(ctx, p.TenantID); err == nil {
		for _, v := range vs {
			plates[v.ID] = v.PlateNumber
		}
	}
	for _, sh := range ships {
		resp.Shipments = append(resp.Shipments, toDriverShipment(sh, plates[sh.VehicleID]))
	}

	if since != nil {
		removed, err := s.q.ListShipmentsUnassignedFromDriverSince(ctx, db.ListShipmentsUnassignedFromDriverSinceParams{
			TenantID: p.TenantID, DriverID: p.UserID, Since: sinceTS,
		})
		if err != nil {
			return err
		}
		for _, id := range removed {
			resp.RemovedShipmentIDs = append(resp.RemovedShipmentIDs, id.String())
		}
	}

	msgSince := resp.ServerTime.Add(-7 * 24 * time.Hour)
	if since != nil {
		msgSince = *since
	}
	msgs, err := s.q.ListDriverMessagesSince(ctx, db.ListDriverMessagesSinceParams{
		TenantID: p.TenantID, DriverID: p.UserID, Since: toTimestamp(msgSince),
	})
	if err != nil {
		return err
	}
	for _, m := range msgs {
		resp.Messages = append(resp.Messages, DriverMessage{
			ID: m.ID.String(), ShipmentID: m.ShipmentID.String(), TrackingNumber: m.TrackingNumber,
			Text: deref(m.Note), From: m.ActorName, At: m.CreatedAt.Time,
		})
	}
	// Messages to every driver on the road belong to no shipment.
	all, err := s.q.ListDriverBroadcastsSince(ctx, db.ListDriverBroadcastsSinceParams{
		TenantID: p.TenantID, DriverID: p.UserID, Since: toTimestamp(msgSince),
	})
	if err != nil {
		return err
	}
	for _, m := range all {
		resp.Messages = append(resp.Messages, DriverMessage{ID: m.ID.String(), Text: m.Body, From: m.SenderName, At: m.CreatedAt.Time})
	}

	u, err := s.q.GetUserByID(ctx, db.GetUserByIDParams{ID: p.UserID, TenantID: p.TenantID})
	if err != nil {
		return err
	}
	resp.Driver = DriverProfile{
		ID: u.ID.String(), Name: u.FullName, Phone: deref(u.Phone), Company: p.TenantName,
		MustChangePIN: u.PasswordResetRequired, SMSNumber: s.cfg.SMSNumber,
	}
	if u.VehicleID.Valid {
		if v, err := s.q.GetVehicleByID(ctx, db.GetVehicleByIDParams{ID: u.VehicleID, TenantID: p.TenantID}); err == nil {
			resp.Driver.Vehicle = v.PlateNumber
		}
	}
	return nil
}

func toDriverShipment(sh db.Shipment, plate string) DriverShipment {
	return DriverShipment{
		ID:                 sh.ID.String(),
		TrackingNumber:     sh.TrackingNumber,
		Ref:                ShortRef(sh.TrackingNumber),
		Status:             sh.Status,
		StatusReason:       deref(sh.StatusReason),
		Priority:           sh.Priority,
		OriginCity:         deref(sh.OriginCity),
		OriginAddress:      sh.OriginAddress,
		OriginLat:          sh.OriginLat,
		OriginLng:          sh.OriginLng,
		DestinationCity:    deref(sh.DestinationCity),
		DestinationAddress: sh.DestinationAddress,
		DestinationLat:     sh.DestinationLat,
		DestinationLng:     sh.DestinationLng,
		DestinationPrecise: sh.DestinationPrecise,
		CustomerName:       sh.CustomerName,
		CustomerPhone:      sh.CustomerPhone,
		Cargo:              deref(sh.CargoDescription),
		WeightKg:           numericOr(sh.CargoWeight),
		Instructions:       deref(sh.SpecialInstructions),
		Vehicle:            plate,
		ETA:                timePtr(sh.EstimatedDelivery),
		PlannedPickup:      timePtr(sh.PlannedPickupAt),
		PickedUpAt:         timePtr(sh.PickedUpAt),
		DeliveredAt:        timePtr(sh.ActualDelivery),
		CreatedAt:          sh.CreatedAt.Time,
		UpdatedAt:          sh.UpdatedAt.Time,
	}
}
