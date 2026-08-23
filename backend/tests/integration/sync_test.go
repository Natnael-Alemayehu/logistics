//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/natnael-alemayehu/logistics/internal/model"
	"github.com/natnael-alemayehu/logistics/internal/service"
)

func TestSync_TrackingEvents(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "sync-events")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "sync-disp@test.com")
	driver := env.CreateTestDriver(ctx, tenant.ID.String(), "0911110000")

	shipmentService := env.GetShipmentService()
	shipment, _ := shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID, service.CreateShipmentInput{
		OriginAddress:      "Addis Ababa",
		DestinationAddress: "Dire Dawa",
		CustomerName:       "Test Customer",
		CustomerPhone:      "0911111111",
		DriverID:           driver.ID,
	}, "127.0.0.1", "test-agent")

	syncService := env.GetSyncService()

	now := time.Now()
	syncResp, err := syncService.Sync(ctx, tenant.ID.String(), driver.ID, model.SyncRequest{
		DeviceID:     "device-001",
		LastSyncAt:   time.Now().Add(-1 * time.Hour),
		BatteryLevel: 85,
		Events: []model.TrackingEventInput{
			{
				ShipmentID: shipment.ID,
				Latitude:   9.0320,
				Longitude:  38.7469,
				Accuracy:   10.5,
				Speed:      45.2,
				Heading:    180.0,
				EventType:  "gps_ping",
				Status:     "on_route",
				RecordedAt: now.Add(-30 * time.Minute),
			},
			{
				ShipmentID: shipment.ID,
				Latitude:   9.0350,
				Longitude:  38.7500,
				Accuracy:   12.0,
				Speed:      50.0,
				Heading:    175.0,
				EventType:  "gps_ping",
				RecordedAt: now.Add(-15 * time.Minute),
			},
		},
	})

	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	if syncResp.EventsReceived != 2 {
		t.Errorf("expected 2 events received, got %d", syncResp.EventsReceived)
	}

	if syncResp.ServerTime.IsZero() {
		t.Error("expected server time to be set")
	}
}

func TestSync_PODs(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "sync-pods")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "sync-pod-disp@test.com")
	driver := env.CreateTestDriver(ctx, tenant.ID.String(), "0922220000")

	shipmentService := env.GetShipmentService()
	shipment, _ := shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID, service.CreateShipmentInput{
		OriginAddress:      "Addis Ababa",
		DestinationAddress: "Bahir Dar",
		CustomerName:       "Test Customer",
		CustomerPhone:      "0922222222",
		DriverID:           driver.ID,
	}, "127.0.0.1", "test-agent")

	syncService := env.GetSyncService()

	now := time.Now()
	syncResp, err := syncService.Sync(ctx, tenant.ID.String(), driver.ID, model.SyncRequest{
		DeviceID:   "device-002",
		LastSyncAt: time.Now().Add(-2 * time.Hour),
		PODs: []model.PODInput{
			{
				ShipmentID:      shipment.ID,
				RecipientName:   "John Doe",
				RecipientPhone:  "0933333333",
				SignatureData:   "base64-signature-data",
				PhotoURLs:       []string{"photo1.jpg", "photo2.jpg"},
				DeliveryAddress: "123 Main St, Bahir Dar",
				DeliveryLat:     11.5744,
				DeliveryLng:     37.3617,
				DeliveryNotes:   "Left at front door",
				RecordedAt:      now.Add(-1 * time.Hour),
			},
		},
	})

	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	if syncResp.EventsReceived != 1 {
		t.Errorf("expected 1 POD received, got %d", syncResp.EventsReceived)
	}
}

func TestSync_StatusUpdates(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "sync-status")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "sync-status-disp@test.com")
	driver := env.CreateTestDriver(ctx, tenant.ID.String(), "0933330000")

	shipmentService := env.GetShipmentService()
	shipment, _ := shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID, service.CreateShipmentInput{
		OriginAddress:      "Addis Ababa",
		DestinationAddress: "Hawassa",
		CustomerName:       "Test Customer",
		CustomerPhone:      "0933333333",
		DriverID:           driver.ID,
	}, "127.0.0.1", "test-agent")

	syncService := env.GetSyncService()

	now := time.Now()
	syncResp, err := syncService.Sync(ctx, tenant.ID.String(), driver.ID, model.SyncRequest{
		DeviceID:   "device-003",
		LastSyncAt: time.Now().Add(-30 * time.Minute),
		Statuses: []model.StatusUpdateInput{
			{
				ShipmentID: shipment.ID,
				Status:     "in_transit",
				Note:       "Package picked up and on the way",
				RecordedAt: now.Add(-20 * time.Minute),
			},
		},
	})

	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	if syncResp.EventsReceived != 1 {
		t.Errorf("expected 1 status update received, got %d", syncResp.EventsReceived)
	}

	updated, _ := shipmentService.GetByID(ctx, tenant.ID.String(), shipment.ID)
	if updated.Status != "in_transit" {
		t.Errorf("expected shipment status 'in_transit', got %s", updated.Status)
	}
}

func TestSync_DeliveredStatus(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "sync-delivered")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "sync-delivered-disp@test.com")
	driver := env.CreateTestDriver(ctx, tenant.ID.String(), "0944440000")

	shipmentService := env.GetShipmentService()
	shipment, _ := shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID, service.CreateShipmentInput{
		OriginAddress:      "Addis Ababa",
		DestinationAddress: "Mekelle",
		CustomerName:       "Test Customer",
		CustomerPhone:      "0944444444",
		DriverID:           driver.ID,
	}, "127.0.0.1", "test-agent")

	_, _ = shipmentService.UpdateStatus(ctx, tenant.ID.String(), dispatcher.ID, "dispatcher", shipment.ID, service.UpdateStatusInput{
		Status: "in_transit",
	}, "127.0.0.1", "test-agent")

	syncService := env.GetSyncService()

	now := time.Now()
	syncResp, err := syncService.Sync(ctx, tenant.ID.String(), driver.ID, model.SyncRequest{
		DeviceID:   "device-004",
		LastSyncAt: time.Now().Add(-1 * time.Hour),
		Statuses: []model.StatusUpdateInput{
			{
				ShipmentID: shipment.ID,
				Status:     "delivered",
				Note:       "Package delivered to customer",
				RecordedAt: now,
			},
		},
		PODs: []model.PODInput{
			{
				ShipmentID:      shipment.ID,
				RecipientName:   "Customer Name",
				DeliveryAddress: "456 Delivery St",
				DeliveryLat:     13.4967,
				DeliveryLng:     39.4647,
				RecordedAt:      now,
			},
		},
	})

	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	if syncResp.EventsReceived != 2 {
		t.Errorf("expected 2 events (status + POD), got %d", syncResp.EventsReceived)
	}

	updated, _ := shipmentService.GetByID(ctx, tenant.ID.String(), shipment.ID)
	if updated.Status != "delivered" {
		t.Errorf("expected shipment status 'delivered', got %s", updated.Status)
	}

	if updated.ActualDelivery == nil {
		t.Error("expected actual delivery time to be set")
	}
}

func TestSync_PullData(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "sync-pull")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "sync-pull-disp@test.com")
	driver := env.CreateTestDriver(ctx, tenant.ID.String(), "0955550000")

	shipmentService := env.GetShipmentService()

	shipment1, _ := shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID, service.CreateShipmentInput{
		OriginAddress:      "Addis Ababa",
		DestinationAddress: "City A",
		CustomerName:       "Customer 1",
		CustomerPhone:      "0955555551",
		DriverID:           driver.ID,
	}, "127.0.0.1", "test-agent")

	shipment2, _ := shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID, service.CreateShipmentInput{
		OriginAddress:      "Addis Ababa",
		DestinationAddress: "City B",
		CustomerName:       "Customer 2",
		CustomerPhone:      "0955555552",
		DriverID:           driver.ID,
	}, "127.0.0.1", "test-agent")

	syncService := env.GetSyncService()

	syncResp, err := syncService.Sync(ctx, tenant.ID.String(), driver.ID, model.SyncRequest{
		DeviceID:   "device-005",
		LastSyncAt: time.Time{},
		Events:     []model.TrackingEventInput{},
		PODs:       []model.PODInput{},
		Statuses:   []model.StatusUpdateInput{},
	})

	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	if len(syncResp.Pull.Shipments) < 2 {
		t.Errorf("expected at least 2 shipments to pull, got %d", len(syncResp.Pull.Shipments))
	}

	pulledIDs := make(map[string]bool)
	for _, s := range syncResp.Pull.Shipments {
		pulledIDs[s.ID] = true
	}

	if !pulledIDs[shipment1.ID] {
		t.Error("expected shipment1 to be in pulled data")
	}
	if !pulledIDs[shipment2.ID] {
		t.Error("expected shipment2 to be in pulled data")
	}
}

func TestSync_PullDataIncremental(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "sync-incremental")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "sync-incr-disp@test.com")
	driver := env.CreateTestDriver(ctx, tenant.ID.String(), "0966660000")

	shipmentService := env.GetShipmentService()

	oldShipment, _ := shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID, service.CreateShipmentInput{
		OriginAddress:      "Addis Ababa",
		DestinationAddress: "Old City",
		CustomerName:       "Old Customer",
		CustomerPhone:      "0966666661",
		DriverID:           driver.ID,
	}, "127.0.0.1", "test-agent")

	lastSyncTime := time.Now().Add(100 * time.Millisecond)

	time.Sleep(150 * time.Millisecond)

	newShipment, _ := shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID, service.CreateShipmentInput{
		OriginAddress:      "Addis Ababa",
		DestinationAddress: "New City",
		CustomerName:       "New Customer",
		CustomerPhone:      "0966666662",
		DriverID:           driver.ID,
	}, "127.0.0.1", "test-agent")

	syncService := env.GetSyncService()

	syncResp, err := syncService.Sync(ctx, tenant.ID.String(), driver.ID, model.SyncRequest{
		DeviceID:   "device-006",
		LastSyncAt: lastSyncTime,
		Events:     []model.TrackingEventInput{},
		PODs:       []model.PODInput{},
		Statuses:   []model.StatusUpdateInput{},
	})

	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	pulledIDs := make(map[string]bool)
	for _, s := range syncResp.Pull.Shipments {
		pulledIDs[s.ID] = true
	}

	if pulledIDs[oldShipment.ID] {
		t.Error("old shipment should not be pulled with incremental sync")
	}

	if !pulledIDs[newShipment.ID] {
		t.Error("new shipment should be pulled with incremental sync")
	}
}

func TestSync_MultipleEventTypes(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "sync-multiple")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "sync-multi-disp@test.com")
	driver := env.CreateTestDriver(ctx, tenant.ID.String(), "0977770000")

	shipmentService := env.GetShipmentService()
	shipment, _ := shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID, service.CreateShipmentInput{
		OriginAddress:      "Addis Ababa",
		DestinationAddress: "Gondar",
		CustomerName:       "Test Customer",
		CustomerPhone:      "0977777777",
		DriverID:           driver.ID,
	}, "127.0.0.1", "test-agent")

	syncService := env.GetSyncService()

	now := time.Now()
	syncResp, err := syncService.Sync(ctx, tenant.ID.String(), driver.ID, model.SyncRequest{
		DeviceID:   "device-007",
		LastSyncAt: time.Now().Add(-3 * time.Hour),
		Events: []model.TrackingEventInput{
			{
				ShipmentID: shipment.ID,
				Latitude:   12.6000,
				Longitude:  37.4667,
				EventType:  "gps_ping",
				RecordedAt: now.Add(-2 * time.Hour),
			},
			{
				ShipmentID: shipment.ID,
				Latitude:   12.6100,
				Longitude:  37.4700,
				EventType:  "checkpoint",
				Status:     "arrived_checkpoint",
				Note:       "Reached Gondar checkpoint",
				RecordedAt: now.Add(-1 * time.Hour),
			},
		},
		Statuses: []model.StatusUpdateInput{
			{
				ShipmentID: shipment.ID,
				Status:     "in_transit",
				Note:       "Started from Addis Ababa",
				RecordedAt: now.Add(-2*time.Hour + 10*time.Minute),
			},
		},
	})

	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	if syncResp.EventsReceived != 3 {
		t.Errorf("expected 3 events received (2 tracking + 1 status), got %d", syncResp.EventsReceived)
	}
}

func TestSync_InvalidShipmentID(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "sync-invalid")
	driver := env.CreateTestDriver(ctx, tenant.ID.String(), "0988880000")

	syncService := env.GetSyncService()

	now := time.Now()
	syncResp, err := syncService.Sync(ctx, tenant.ID.String(), driver.ID, model.SyncRequest{
		DeviceID:   "device-008",
		LastSyncAt: time.Now().Add(-1 * time.Hour),
		Events: []model.TrackingEventInput{
			{
				ShipmentID: "00000000-0000-0000-0000-000000000000",
				Latitude:   9.0320,
				Longitude:  38.7469,
				EventType:  "gps_ping",
				RecordedAt: now,
			},
		},
	})

	if err != nil {
		t.Fatalf("sync should not fail for invalid shipment: %v", err)
	}

	if syncResp.EventsReceived != 0 {
		t.Errorf("expected 0 events received for invalid shipment, got %d", syncResp.EventsReceived)
	}
}

func TestSync_TenantIsolation(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant1 := env.CreateTestTenant(ctx, "sync-tenant1")
	tenant2 := env.CreateTestTenant(ctx, "sync-tenant2")

	dispatcher1 := env.CreateTestDispatcher(ctx, tenant1.ID.String(), "sync-t1@test.com")
	driver1 := env.CreateTestDriver(ctx, tenant1.ID.String(), "0999990001")
	driver2 := env.CreateTestDriver(ctx, tenant2.ID.String(), "0999990002")

	shipmentService := env.GetShipmentService()
	shipment1, _ := shipmentService.Create(ctx, tenant1.ID.String(), dispatcher1.ID, service.CreateShipmentInput{
		OriginAddress:      "City A",
		DestinationAddress: "City B",
		CustomerName:       "Tenant1 Customer",
		CustomerPhone:      "0999999001",
		DriverID:           driver1.ID,
	}, "127.0.0.1", "test-agent")

	syncService := env.GetSyncService()

	now := time.Now()
	syncResp, err := syncService.Sync(ctx, tenant2.ID.String(), driver2.ID, model.SyncRequest{
		DeviceID:   "device-009",
		LastSyncAt: time.Time{},
		Events: []model.TrackingEventInput{
			{
				ShipmentID: shipment1.ID,
				Latitude:   9.0320,
				Longitude:  38.7469,
				EventType:  "gps_ping",
				RecordedAt: now,
			},
		},
	})

	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	if syncResp.EventsReceived != 0 {
		t.Errorf("tenant2 driver should not be able to sync events for tenant1 shipment, got %d", syncResp.EventsReceived)
	}

	for _, s := range syncResp.Pull.Shipments {
		if s.TenantID == tenant1.ID.String() {
			t.Error("tenant2 driver should not receive tenant1 shipments")
		}
	}
}

func TestSync_HTTP(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "sync-http")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "sync-http-disp@test.com")
	driver := env.CreateTestDriver(ctx, tenant.ID.String(), "0900000001")

	shipmentService := env.GetShipmentService()
	shipment, _ := shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID, service.CreateShipmentInput{
		OriginAddress:      "Addis Ababa",
		DestinationAddress: "Dire Dawa",
		CustomerName:       "Test Customer",
		CustomerPhone:      "0900000001",
		DriverID:           driver.ID,
	}, "127.0.0.1", "test-agent")

	authService := env.GetAuthService()
	loginResult, _ := authService.DriverLogin(ctx, service.DriverLoginInput{
		Phone: "0900000001",
		PIN:   "1234",
	}, "127.0.0.1", "test-agent")

	now := time.Now().Format(time.RFC3339)
	body := fmt.Sprintf(`{
		"device_id": "device-http",
		"last_sync_at": "%s",
		"battery_level": 80,
		"events": [
			{
				"shipment_id": "%s",
				"latitude": 9.0320,
				"longitude": 38.7469,
				"accuracy": 10,
				"speed": 45,
				"heading": 180,
				"event_type": "gps_ping",
				"recorded_at": "%s"
			}
		],
		"pods": [],
		"statuses": []
	}`, time.Now().Add(-1*time.Hour).Format(time.RFC3339), shipment.ID, now)

	req := httptest.NewRequest(http.MethodPost, "/sync", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+loginResult.AccessToken)
	rec := httptest.NewRecorder()

	env.GetRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &response)

	data, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data in response")
	}

	eventsReceived := int(data["events_received"].(float64))
	if eventsReceived != 1 {
		t.Errorf("expected 1 event received, got %d", eventsReceived)
	}
}

func TestSync_EmptySync(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "sync-empty")
	driver := env.CreateTestDriver(ctx, tenant.ID.String(), "0900000002")

	syncService := env.GetSyncService()

	syncResp, err := syncService.Sync(ctx, tenant.ID.String(), driver.ID, model.SyncRequest{
		DeviceID:   "device-empty",
		LastSyncAt: time.Now(),
		Events:     []model.TrackingEventInput{},
		PODs:       []model.PODInput{},
		Statuses:   []model.StatusUpdateInput{},
	})

	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	if syncResp.EventsReceived != 0 {
		t.Errorf("expected 0 events received for empty sync, got %d", syncResp.EventsReceived)
	}
}

func TestSync_LargeBatch(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "sync-batch")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "sync-batch-disp@test.com")
	driver := env.CreateTestDriver(ctx, tenant.ID.String(), "0900000003")

	shipmentService := env.GetShipmentService()
	shipment, _ := shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID, service.CreateShipmentInput{
		OriginAddress:      "Addis Ababa",
		DestinationAddress: "Dire Dawa",
		CustomerName:       "Test Customer",
		CustomerPhone:      "0900000003",
		DriverID:           driver.ID,
	}, "127.0.0.1", "test-agent")

	syncService := env.GetSyncService()

	var events []model.TrackingEventInput
	now := time.Now()
	for i := 0; i < 100; i++ {
		events = append(events, model.TrackingEventInput{
			ShipmentID: shipment.ID,
			Latitude:   9.0320 + float64(i)*0.001,
			Longitude:  38.7469 + float64(i)*0.001,
			Accuracy:   10.0,
			Speed:      45.0,
			Heading:    180.0,
			EventType:  "gps_ping",
			RecordedAt: now.Add(-time.Duration(100-i) * time.Minute),
		})
	}

	syncResp, err := syncService.Sync(ctx, tenant.ID.String(), driver.ID, model.SyncRequest{
		DeviceID:   "device-batch",
		LastSyncAt: time.Now().Add(-2 * time.Hour),
		Events:     events,
	})

	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	if syncResp.EventsReceived != 100 {
		t.Errorf("expected 100 events received, got %d", syncResp.EventsReceived)
	}
}

func TestSync_DeviceMetadata(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "sync-metadata")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "sync-meta-disp@test.com")
	driver := env.CreateTestDriver(ctx, tenant.ID.String(), "0900000004")

	shipmentService := env.GetShipmentService()
	shipment, _ := shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID, service.CreateShipmentInput{
		OriginAddress:      "Addis Ababa",
		DestinationAddress: "Dire Dawa",
		CustomerName:       "Test Customer",
		CustomerPhone:      "0900000004",
		DriverID:           driver.ID,
	}, "127.0.0.1", "test-agent")

	syncService := env.GetSyncService()

	now := time.Now()
	syncResp, err := syncService.Sync(ctx, tenant.ID.String(), driver.ID, model.SyncRequest{
		DeviceID:         "device-metadata",
		LastSyncAt:       time.Now().Add(-1 * time.Hour),
		BatteryLevel:     75,
		StorageRemaining: 50000,
		Events: []model.TrackingEventInput{
			{
				ShipmentID: shipment.ID,
				Latitude:   9.0320,
				Longitude:  38.7469,
				EventType:  "gps_ping",
				RecordedAt: now,
			},
		},
	})

	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	if syncResp.EventsReceived != 1 {
		t.Errorf("expected 1 event received, got %d", syncResp.EventsReceived)
	}
}
