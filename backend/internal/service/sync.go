package service

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/kiflet/internal/db"
	"github.com/natnael-alemayehu/kiflet/internal/geo"
	"github.com/natnael-alemayehu/kiflet/internal/model"
	"github.com/natnael-alemayehu/kiflet/pkg/metrics"
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
	// PowerSaving says the phone records a position only every 15 minutes,
	// by the driver's choice or because its battery is low.
	PowerSaving bool `json:"power_saving"`
	// Settings holds the settings the driver changed on this phone since
	// the last sync.
	Settings *SettingsChange `json:"settings,omitempty"`
	// Since is the cursor from the previous response; empty on first sync.
	Since *time.Time `json:"since"`
	// ReceivedAt is when the request reached the server, set by the handler
	// before it reads the body: the moment to compare DeviceTime with.
	ReceivedAt time.Time `json:"-"`

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
	ClientID       string   `json:"client_id"`
	ShipmentID     string   `json:"shipment_id"`
	RecipientName  string   `json:"recipient_name"`
	RecipientPhone string   `json:"recipient_phone"`
	Signature      string   `json:"signature"`
	Photos         []string `json:"photos"`
	Notes          string   `json:"notes"`
	Lat            *float64 `json:"lat"`
	Lng            *float64 `json:"lng"`
	// LocationReason is why the driver delivered away from the destination
	// after the app warned them.
	LocationReason string    `json:"location_reason"`
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
	// More says there were more messages than one response carries: the
	// cursor stops at the last one sent, and the app asks again at once.
	More bool `json:"more,omitempty"`
}

// DriverShipment is a shipment as the driver app holds it.
type DriverShipment struct {
	ID                 string   `json:"id"`
	TrackingNumber     string   `json:"tracking_number"`
	Ref                string   `json:"ref"`
	Status             string   `json:"status"`
	StatusReason       string   `json:"status_reason,omitempty"`
	Priority           string   `json:"priority"`
	OriginCity         string   `json:"origin_city"`
	OriginAddress      string   `json:"origin_address"`
	OriginLat          *float64 `json:"origin_lat"`
	OriginLng          *float64 `json:"origin_lng"`
	DestinationCity    string   `json:"destination_city"`
	DestinationAddress string   `json:"destination_address"`
	DestinationLat     *float64 `json:"destination_lat"`
	DestinationLng     *float64 `json:"destination_lng"`
	DestinationPrecise bool     `json:"destination_precise"`
	// ToleranceM is how far from the destination a delivery may be recorded
	// before the app asks why.
	ToleranceM    float64    `json:"tolerance_m"`
	CustomerName  string     `json:"customer_name"`
	CustomerPhone string     `json:"customer_phone"`
	Cargo         string     `json:"cargo"`
	WeightKg      float64    `json:"weight_kg"`
	Instructions  string     `json:"instructions"`
	Vehicle       string     `json:"vehicle"`
	ETA           *time.Time `json:"eta"`
	PlannedPickup *time.Time `json:"planned_pickup"`
	PickedUpAt    *time.Time `json:"picked_up_at"`
	DeliveredAt   *time.Time `json:"delivered_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	// RecentRecipients are people who signed for this customer before,
	// newest first, to suggest on the delivery screen.
	RecentRecipients []string `json:"recent_recipients"`
}

// DriverMessage is a note from the office, shown in the app.
type DriverMessage struct {
	ID             string `json:"id"`
	ShipmentID     string `json:"shipment_id"`
	TrackingNumber string `json:"tracking_number"`
	// Text is the message as shown, in English for what the office did.
	Text string `json:"text"`
	// Kind is "cancelled" or "reopened" when the office did that to the
	// shipment, for the app to say in the driver's language; Note is then
	// what the office wrote, if anything.
	Kind string    `json:"kind,omitempty"`
	Note string    `json:"note,omitempty"`
	From string    `json:"from"`
	At   time.Time `json:"at"`
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
	// OfficePhone is the company's number, for the driver to call.
	OfficePhone string         `json:"office_phone"`
	Settings    DriverSettings `json:"settings"`
}

// DriverSettings are the driver app's settings, kept on the server so they
// follow the driver to a new phone (FR-026 AC-026.6).
type DriverSettings struct {
	// Language is "en" or "am", or empty until the driver chooses one.
	Language     string `json:"language"`
	BatterySaver bool   `json:"battery_saver"`
	Vibrate      bool   `json:"vibrate"`
}

// SettingsChange is what the driver changed on the phone. Settings left out
// stay as they are, so a language picked at sign-in on a new phone doesn't
// undo the others.
type SettingsChange struct {
	Language     *string `json:"language"`
	BatterySaver *bool   `json:"battery_saver"`
	Vibrate      *bool   `json:"vibrate"`
}

// recentRecipients is how many earlier recipients the app suggests.
const recentRecipients = 5

// Sync limits per request, beyond which the device sends the rest next time.
const (
	maxSyncStatuses  = 200
	maxSyncPODs      = 20
	maxSyncLocations = 5000
	maxSyncMessages  = 100
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
	// and ours when the request arrived corrects every timestamp in it,
	// however small: a phone a minute slow would otherwise report a move
	// made just after the office's as made before it, and have it refused
	// as out of date. Arrived means when the request's headers did, before
	// a body with photos had uploaded, so a slow upload doesn't pass for a
	// slow clock.
	arrived := req.ReceivedAt
	if arrived.IsZero() || arrived.After(now) || now.Sub(arrived) > 10*time.Minute {
		arrived = now
	}
	var offset time.Duration
	if req.DeviceTime != nil {
		offset = arrived.Sub(*req.DeviceTime)
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
		// Points recorded offline for a shipment handed to another driver
		// meanwhile are still this driver's trip: kept, if it was theirs.
		checked := map[pgtype.UUID]bool{}
		for _, l := range locs {
			id := toUUID(l.ShipmentID)
			if !id.Valid || allowed[id] || checked[id] {
				continue
			}
			checked[id] = true
			had, err := s.q.WasAssignedTo(ctx, db.WasAssignedToParams{TenantID: p.TenantID, ShipmentID: id, DriverID: p.UserID})
			if err != nil {
				return nil, err
			}
			allowed[id] = had
		}
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

	for _, r := range resp.Results {
		metrics.SyncChanges.WithLabelValues(r.Kind, syncOutcome(r)).Inc()
	}
	metrics.SyncLocations.Add(float64(resp.LocationsStored))

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
		clockOffset = ptr(int32(offset.Seconds()))
	}
	if err := s.q.UpsertSyncMetadata(ctx, db.UpsertSyncMetadataParams{
		DriverID: p.UserID, DeviceID: toText(truncate(req.DeviceID, 100)), AppVersion: toText(truncate(req.AppVersion, 40)),
		BatteryLevel: battery, PendingSyncCount: pending, StorageUsedKb: storageKB,
		ClockOffsetSeconds: clockOffset, TrackingPaused: req.TrackingPaused, PowerSaving: req.PowerSaving,
	}); err != nil {
		s.log.Error().Err(err).Msg("upsert sync metadata")
	}
	if st := req.Settings; st != nil {
		lang := st.Language
		if lang != nil && *lang != "en" && *lang != "am" {
			lang = nil
		}
		if err := s.q.SetDriverAppSettings(ctx, db.SetDriverAppSettingsParams{
			Language: lang, BatterySaver: st.BatterySaver, Vibrate: st.Vibrate, ID: p.UserID, TenantID: p.TenantID,
		}); err != nil {
			return nil, err
		}
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
		Lat: in.Lat, Lng: in.Lng, LocationReason: in.LocationReason, RecordedAt: at, ClientID: in.ClientID,
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
// syncOutcome names a result for /metrics.
func syncOutcome(r SyncResult) string {
	switch {
	case r.OK:
		return "applied"
	case r.Retry:
		return "retry"
	}
	return "refused"
}

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
	phones := make([]string, 0, len(ships))
	for _, sh := range ships {
		phones = append(phones, sh.CustomerPhone)
	}
	signed := map[string][]string{}
	if len(phones) > 0 {
		rows, err := s.q.ListRecentRecipients(ctx, db.ListRecentRecipientsParams{TenantID: p.TenantID, Phones: phones})
		if err != nil {
			return err
		}
		for _, r := range rows {
			if len(signed[r.CustomerPhone]) < recentRecipients {
				signed[r.CustomerPhone] = append(signed[r.CustomerPhone], r.RecipientName)
			}
		}
	}
	for _, sh := range ships {
		ds := toDriverShipment(sh, plates[sh.VehicleID])
		ds.RecentRecipients = signed[sh.CustomerPhone]
		if ds.RecentRecipients == nil {
			ds.RecentRecipients = []string{}
		}
		resp.Shipments = append(resp.Shipments, ds)
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
		dm := DriverMessage{
			ID: m.ID.String(), ShipmentID: m.ShipmentID.String(), TrackingNumber: m.TrackingNumber,
			Text: deref(m.Note), From: m.ActorName, At: m.CreatedAt.Time,
		}
		if m.EventType == "status_changed" {
			// The office cancelled or reopened the shipment: said even
			// when it wrote nothing.
			if deref(m.ToStatus) == model.StatusCancelled {
				dm.Kind, dm.Note = "cancelled", dm.Text
				dm.Text = "The office cancelled this shipment."
			} else {
				dm.Kind = "reopened"
				dm.Text = "The office reopened this shipment."
			}
			if dm.Note != "" {
				dm.Text += " " + dm.Note
			}
		}
		resp.Messages = append(resp.Messages, dm)
	}
	if len(msgs) == maxDriverMessages {
		s.cutCursor(resp, msgSince, msgs[len(msgs)-1].CreatedAt.Time)
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
	if len(all) == maxDriverBroadcasts {
		s.cutCursor(resp, msgSince, all[len(all)-1].CreatedAt.Time)
	}

	u, err := s.q.GetUserByID(ctx, db.GetUserByIDParams{ID: p.UserID, TenantID: p.TenantID})
	if err != nil {
		return err
	}
	resp.Driver = DriverProfile{
		ID: u.ID.String(), Name: u.FullName, Phone: deref(u.Phone), Company: p.TenantName,
		MustChangePIN: u.PasswordResetRequired, SMSNumber: s.cfg.SMSNumber,
		Settings: DriverSettings{Language: deref(u.Language), BatterySaver: u.AppBatterySaver, Vibrate: u.AppVibrate},
	}
	if t, err := s.q.GetTenantByID(ctx, p.TenantID); err == nil {
		resp.Driver.OfficePhone = deref(t.ContactPhone)
	}
	if u.VehicleID.Valid {
		if v, err := s.q.GetVehicleByID(ctx, db.GetVehicleByIDParams{ID: u.VehicleID, TenantID: p.TenantID}); err == nil {
			resp.Driver.Vehicle = v.PlateNumber
		}
	}
	return nil
}

// The most messages and broadcasts one sync returns: the LIMITs of
// ListDriverMessagesSince and ListDriverBroadcastsSince.
const (
	maxDriverMessages   = 200
	maxDriverBroadcasts = 50
)

// cutCursor stops the cursor just before the last of a full page of
// messages, so the next sync, made at once, carries on from there instead
// of skipping the rest. Messages seen twice are dropped by the app. A page
// whose rows all share one time moves on past it, or it would come back
// for ever.
func (s *SyncService) cutCursor(resp *SyncResponse, since, last time.Time) {
	c := last.Add(-time.Microsecond)
	if !c.After(since) {
		c = last
	}
	if c.Before(resp.Cursor) {
		resp.Cursor = c
	}
	resp.More = true
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
		ToleranceM:         geo.DeliveryTolerance(sh.DestinationPrecise),
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
