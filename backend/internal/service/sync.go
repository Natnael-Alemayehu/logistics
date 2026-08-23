package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/natnael-alemayehu/logistics/internal/db"
	"github.com/natnael-alemayehu/logistics/internal/model"
	"github.com/natnael-alemayehu/logistics/pkg/storage"
	"github.com/natnael-alemayehu/logistics/pkg/websocket"
	"github.com/rs/zerolog"
)

// pgUniqueViolation is the SQLSTATE Postgres reports for a unique constraint
// violation. For sync it means the item is already stored, which is a success
// for a client replaying a batch whose response it never saw.
const pgUniqueViolation = "23505"

type SyncService struct {
	queries      *db.Queries
	shipment     *ShipmentService
	eventService *EventService
	podStorage   *PODStorageService
	logger       zerolog.Logger
}

func NewSyncService(
	queries *db.Queries,
	shipment *ShipmentService,
	eventService *EventService,
	podStorage *PODStorageService,
	logger zerolog.Logger,
) *SyncService {
	return &SyncService{
		queries:      queries,
		shipment:     shipment,
		eventService: eventService,
		podStorage:   podStorage,
		logger:       logger,
	}
}

var (
	// errShipmentNotInTenant means the shipment does not exist within the
	// submitting driver's tenant.
	errShipmentNotInTenant = errors.New("shipment does not belong to this tenant")
	// errShipmentNotAssigned means the shipment exists but belongs to a different
	// driver.
	errShipmentNotAssigned = errors.New("shipment is not assigned to this driver")
)

// syncActor is the authenticated driver a sync request belongs to. The
// identifiers are parsed once so that each item does not revalidate them.
type syncActor struct {
	tenantID   string
	driverID   string
	tenantUUID pgtype.UUID
	driverUUID pgtype.UUID
}

// Sync applies a batch of offline changes and returns what the client should
// pull in response.
//
// Every submitted item gets its own result. A failure to store one item never
// fails the request, because the rest of the batch is independent and a client
// that received no results at all would have to retry everything. The client is
// expected to mark a record synced only when its result says accepted or
// duplicate; anything else stays on the device.
func (s *SyncService) Sync(ctx context.Context, tenantID, driverID string, req model.SyncRequest) (*model.SyncResponse, error) {
	tenantUUID, err := parseUUID(tenantID)
	if err != nil {
		return nil, fmt.Errorf("invalid tenant id: %w", err)
	}
	driverUUID, err := parseUUID(driverID)
	if err != nil {
		return nil, fmt.Errorf("invalid driver id: %w", err)
	}

	actor := syncActor{
		tenantID:   tenantID,
		driverID:   driverID,
		tenantUUID: tenantUUID,
		driverUUID: driverUUID,
	}

	now := time.Now()
	statuses := req.GetStatusUpdates()

	resp := &model.SyncResponse{
		ServerTime:         now,
		SyncTime:           now,
		Events:             make([]model.SyncItemResult, len(req.Events)),
		PODs:               make([]model.SyncItemResult, len(req.PODs)),
		Statuses:           make([]model.SyncItemResult, len(statuses)),
		DeletedShipmentIDs: []string{},
	}

	for i, event := range req.Events {
		resp.Events[i] = s.applyTrackingEvent(ctx, actor, req, i, event)
	}
	for i, pod := range req.PODs {
		resp.PODs[i] = s.applyPOD(ctx, actor, i, pod)
	}
	for i, status := range statuses {
		resp.Statuses[i] = s.applyStatusUpdate(ctx, actor, i, status)
	}

	resp.EventsReceived = countAccepted(resp.Events) + countAccepted(resp.PODs) + countAccepted(resp.Statuses)

	shipments, err := s.queries.GetShipmentsForSync(ctx, db.GetShipmentsForSyncParams{
		DriverID:  driverUUID,
		TenantID:  tenantUUID,
		UpdatedAt: toTimestamp(req.LastSyncAt),
	})
	if err != nil {
		// The accepted items above are committed, so the results must still reach
		// the client. Returning an error here would discard them and force a
		// retry of the whole batch.
		s.logger.Error().Err(err).
			Str("tenant_id", tenantID).
			Str("driver_id", driverID).
			Msg("sync: failed to load shipments to pull")

		return resp, nil
	}

	for i := range shipments {
		resp.Pull.Shipments = append(resp.Pull.Shipments, *dbShipmentToModel(&shipments[i]))
	}

	return resp, nil
}

// authorizeShipment confirms the shipment exists within the actor's tenant and
// is assigned to the driver submitting the item.
//
// Nothing in the insert itself enforces this: tenant_id is taken from the
// caller's token while shipment_id comes from the request body, so without this
// check a driver could attach tracking events or a proof of delivery to another
// tenant's shipment. The row would then be filed under the caller's tenant while
// referencing a shipment belonging to someone else, corrupting both tenants'
// records. Status updates already get this via ShipmentService.UpdateStatus.
func (s *SyncService) authorizeShipment(ctx context.Context, actor syncActor, shipmentUUID pgtype.UUID) error {
	shipment, err := s.queries.GetShipmentByID(ctx, db.GetShipmentByIDParams{
		ID:       shipmentUUID,
		TenantID: actor.tenantUUID,
	})
	if err != nil {
		return errShipmentNotInTenant
	}

	if !shipment.DriverID.Valid || shipment.DriverID.Bytes != actor.driverUUID.Bytes {
		return errShipmentNotAssigned
	}

	return nil
}

// rejectUnauthorized maps an authorizeShipment error to an item result.
func rejectUnauthorized(index int, clientID string, err error) model.SyncItemResult {
	if errors.Is(err, errShipmentNotAssigned) {
		return rejectItem(index, clientID, model.SyncErrNotAssigned, err.Error())
	}
	return rejectItem(index, clientID, model.SyncErrInvalidShipmentID, err.Error())
}

func (s *SyncService) applyTrackingEvent(
	ctx context.Context,
	actor syncActor,
	req model.SyncRequest,
	index int,
	event model.TrackingEventInput,
) model.SyncItemResult {
	shipmentUUID, err := parseUUID(event.ShipmentID)
	if err != nil {
		return rejectItem(index, event.ClientID, model.SyncErrInvalidShipmentID, err.Error())
	}

	if err := s.authorizeShipment(ctx, actor, shipmentUUID); err != nil {
		s.logger.Warn().Err(err).
			Str("tenant_id", actor.tenantID).
			Str("driver_id", actor.driverID).
			Str("shipment_id", event.ShipmentID).
			Msg("sync: refused tracking event for unauthorized shipment")

		return rejectUnauthorized(index, event.ClientID, err)
	}

	_, err = s.queries.CreateTrackingEvent(ctx, db.CreateTrackingEventParams{
		TenantID:       actor.tenantUUID,
		ShipmentID:     shipmentUUID,
		DriverID:       actor.driverUUID,
		StMakepoint:    event.Longitude,
		StMakepoint_2:  event.Latitude,
		AccuracyMeters: toNumeric(event.Accuracy),
		SpeedKph:       toNumeric(event.Speed),
		Heading:        toNumeric(event.Heading),
		EventType:      event.EventType,
		Status:         toText(event.Status),
		Note:           toText(event.Note),
		RecordedAt:     toTimestamp(event.RecordedAt),
		DeviceID:       toText(req.DeviceID),
		BatteryLevel:   toInt32Ptr(req.BatteryLevel),
		ClientID:       toText(event.ClientID),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return duplicateItem(index, event.ClientID)
		}

		s.logger.Warn().Err(err).
			Str("tenant_id", actor.tenantID).
			Str("driver_id", actor.driverID).
			Str("shipment_id", event.ShipmentID).
			Str("event_type", event.EventType).
			Msg("sync: rejected tracking event")

		return rejectItem(index, event.ClientID, model.SyncErrInvalidPayload, err.Error())
	}

	if s.eventService != nil {
		s.eventService.PublishTrackingUpdate(
			actor.tenantID,
			actor.driverID,
			event.ShipmentID,
			websocket.Location{Lat: event.Latitude, Lng: event.Longitude},
			event.Status,
		)
	}

	return acceptItem(index, event.ClientID)
}

func (s *SyncService) applyPOD(
	ctx context.Context,
	actor syncActor,
	index int,
	pod model.PODInput,
) model.SyncItemResult {
	shipmentUUID, err := parseUUID(pod.ShipmentID)
	if err != nil {
		return rejectItem(index, pod.ClientID, model.SyncErrInvalidShipmentID, err.Error())
	}

	if err := s.authorizeShipment(ctx, actor, shipmentUUID); err != nil {
		s.logger.Warn().Err(err).
			Str("tenant_id", actor.tenantID).
			Str("driver_id", actor.driverID).
			Str("shipment_id", pod.ShipmentID).
			Msg("sync: refused POD for unauthorized shipment")

		return rejectUnauthorized(index, pod.ClientID, err)
	}

	// Photos and the signature arrive base64-encoded and must be uploaded before
	// the record is written, so that photo_urls holds storage keys rather than
	// the device-local file:// paths that used to be stored verbatim.
	signatureURL, photoURLs, err := s.uploadPODMedia(ctx, actor, pod)
	if err != nil {
		s.logger.Error().Err(err).
			Str("tenant_id", actor.tenantID).
			Str("shipment_id", pod.ShipmentID).
			Msg("sync: failed to store POD media")

		return rejectItem(index, pod.ClientID, model.SyncErrStorageFailed, err.Error())
	}

	created, err := s.queries.CreatePOD(ctx, db.CreatePODParams{
		TenantID:               actor.tenantUUID,
		ShipmentID:             shipmentUUID,
		DriverID:               actor.driverUUID,
		RecipientName:          pod.RecipientName,
		RecipientPhone:         toText(pod.RecipientPhone),
		SignatureData:          toText(pod.SignatureData),
		SignatureUrl:           toText(signatureURL),
		PhotoUrls:              photoURLs,
		DeliveryAddress:        toText(pod.DeliveryAddress),
		StMakepoint:            pod.DeliveryLng,
		StMakepoint_2:          pod.DeliveryLat,
		DeliveryNotes:          toText(pod.DeliveryNotes),
		LocationVerified:       toBoolPtr(pod.LocationVerified),
		LocationMismatchMeters: toNumeric(pod.LocationMismatchMeters),
		RecordedAt:             toTimestamp(pod.RecordedAt),
	})
	if err != nil {
		// proof_of_deliveries.shipment_id is unique, so this is the normal
		// outcome when a driver's device replays a POD it already delivered.
		if isUniqueViolation(err) {
			return duplicateItem(index, pod.ClientID)
		}

		s.logger.Warn().Err(err).
			Str("tenant_id", actor.tenantID).
			Str("shipment_id", pod.ShipmentID).
			Msg("sync: rejected POD")

		return rejectItem(index, pod.ClientID, model.SyncErrInvalidPayload, err.Error())
	}

	if s.eventService != nil {
		trackingNumber := ""
		shipment, err := s.queries.GetShipmentByID(ctx, db.GetShipmentByIDParams{
			ID:       shipmentUUID,
			TenantID: actor.tenantUUID,
		})
		if err == nil {
			trackingNumber = shipment.TrackingNumber
		}

		signature := ""
		if created.SignatureUrl != nil {
			signature = *created.SignatureUrl
		}
		photo := ""
		if len(created.PhotoUrls) > 0 {
			photo = created.PhotoUrls[0]
		}

		s.eventService.PublishDeliveryComplete(
			actor.tenantID,
			pod.ShipmentID,
			trackingNumber,
			actor.driverID,
			photo,
			signature,
		)
	}

	return acceptItem(index, pod.ClientID)
}

// uploadPODMedia stores the POD's signature and photos, returning the values to
// persist alongside the record.
//
// The photo field has carried three different things over time, so each value is
// classified rather than uploaded blindly:
//
//   - base64 image data, which is uploaded and replaced by its storage key;
//   - a URL or key from an already-completed upload, which passes through;
//   - a device-local file:// path, which is meaningless off the device. Storing
//     those verbatim is the bug this replaces, so they are rejected outright
//     rather than persisted as unusable evidence.
func (s *SyncService) uploadPODMedia(
	ctx context.Context,
	actor syncActor,
	pod model.PODInput,
) (signatureURL string, photoURLs []string, err error) {
	if len(pod.PhotoURLs) > storage.MaxPhotosPerPOD {
		return "", nil, fmt.Errorf("POD carries %d photos, at most %d are accepted",
			len(pod.PhotoURLs), storage.MaxPhotosPerPOD)
	}

	photoURLs = make([]string, 0, len(pod.PhotoURLs))
	for _, photo := range pod.PhotoURLs {
		if isDeviceLocalPath(photo) {
			return "", nil, fmt.Errorf("photo %q is a device-local path and cannot be stored", photo)
		}

		if s.podStorage == nil || !isImagePayload(photo) {
			photoURLs = append(photoURLs, photo)
			continue
		}

		key, uploadErr := s.podStorage.UploadPhoto(ctx, pod.ShipmentID, photo, len(photoURLs))
		if uploadErr != nil {
			return "", nil, fmt.Errorf("failed to upload photo: %w", uploadErr)
		}
		photoURLs = append(photoURLs, key)
	}

	if isDeviceLocalPath(pod.SignatureData) {
		return "", nil, fmt.Errorf("signature is a device-local path and cannot be stored")
	}

	if s.podStorage != nil && isImagePayload(pod.SignatureData) {
		signatureURL, err = s.podStorage.UploadSignature(ctx, pod.ShipmentID, pod.SignatureData)
		if err != nil {
			return "", nil, fmt.Errorf("failed to upload signature: %w", err)
		}
	}

	return signatureURL, photoURLs, nil
}

// isDeviceLocalPath reports whether value refers to a file on the client device.
func isDeviceLocalPath(value string) bool {
	return strings.HasPrefix(value, "file://")
}

// minEncodedImageBytes is the size below which a decodable value is assumed to
// be an identifier rather than an image. Even a heavily compressed delivery
// photo or signature is far larger than this, while keys and filenames are far
// smaller.
const minEncodedImageBytes = 512

// isImagePayload reports whether value carries image data to upload, as opposed
// to a URL or storage key that has already been uploaded.
func isImagePayload(value string) bool {
	if value == "" {
		return false
	}
	if strings.HasPrefix(value, "data:") {
		return true
	}

	decoded, err := base64.StdEncoding.DecodeString(value)
	return err == nil && len(decoded) >= minEncodedImageBytes
}

func (s *SyncService) applyStatusUpdate(
	ctx context.Context,
	actor syncActor,
	index int,
	status model.StatusUpdateInput,
) model.SyncItemResult {
	if _, err := parseUUID(status.ShipmentID); err != nil {
		return rejectItem(index, status.ClientID, model.SyncErrInvalidShipmentID, err.Error())
	}

	_, err := s.shipment.UpdateStatus(ctx, actor.tenantID, actor.driverID, "driver", status.ShipmentID, UpdateStatusInput{
		Status:       status.Status,
		StatusNote:   status.Note,
		StatusReason: status.Reason,
		RecordedAt:   status.GetRecordedAt(),
	}, "", "")
	if err != nil {
		switch {
		case errors.Is(err, ErrStaleUpdate):
			// The server already knows something newer. Retrying will never help,
			// so tell the client to stop holding it.
			return rejectItem(index, status.ClientID, model.SyncErrStale, err.Error())
		case errors.Is(err, ErrNotAssigned):
			return rejectItem(index, status.ClientID, model.SyncErrNotAssigned, err.Error())
		}

		s.logger.Warn().Err(err).
			Str("tenant_id", actor.tenantID).
			Str("driver_id", actor.driverID).
			Str("shipment_id", status.ShipmentID).
			Str("status", status.Status).
			Msg("sync: rejected status update")

		return rejectItem(index, status.ClientID, model.SyncErrInternal, err.Error())
	}

	return acceptItem(index, status.ClientID)
}

func acceptItem(index int, clientID string) model.SyncItemResult {
	return model.SyncItemResult{
		Index:    index,
		ClientID: clientID,
		Status:   model.SyncItemAccepted,
	}
}

func duplicateItem(index int, clientID string) model.SyncItemResult {
	return model.SyncItemResult{
		Index:    index,
		ClientID: clientID,
		Status:   model.SyncItemDuplicate,
	}
}

func rejectItem(index int, clientID, code, message string) model.SyncItemResult {
	return model.SyncItemResult{
		Index:    index,
		ClientID: clientID,
		Status:   model.SyncItemRejected,
		Code:     code,
		Message:  message,
	}
}

func countAccepted(results []model.SyncItemResult) int {
	n := 0
	for _, result := range results {
		if result.Status == model.SyncItemAccepted {
			n++
		}
	}
	return n
}

// isUniqueViolation reports whether err is a Postgres unique constraint
// violation.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation
}
