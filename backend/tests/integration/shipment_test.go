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

	"github.com/natnael-alemayehu/logistics/internal/service"
)

func TestShipmentCreation_Success(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()
	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "shipment-create")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "ship-create@test.com")
	shipmentService := env.GetShipmentService()
	shipment, err := shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID, service.CreateShipmentInput{
		OriginAddress:      "Addis Ababa",
		DestinationAddress: "Dire Dawa",
		CustomerName:       "Test Customer",
		CustomerPhone:      "0911111111",
		CargoDescription:   "Electronics",
		CargoWeight:        150.5,
	}, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("failed to create shipment: %v", err)
	}
	if shipment.Status != "pending" {
		t.Errorf("expected status pending, got %s", shipment.Status)
	}
	if shipment.TrackingNumber == "" {
		t.Error("expected tracking number to be set")
	}
	if shipment.CustomerName != "Test Customer" {
		t.Errorf("expected customer name 'Test Customer', got %s", shipment.CustomerName)
	}
}
func TestShipmentCreation_WithDriver(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()
	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "shipment-with-driver")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "ship-driver@test.com")
	driver := env.CreateTestDriver(ctx, tenant.ID.String(), "0911112222")
	shipmentService := env.GetShipmentService()
	shipment, err := shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID, service.CreateShipmentInput{
		OriginAddress:      "Addis Ababa",
		DestinationAddress: "Bahir Dar",
		CustomerName:       "Test Customer",
		CustomerPhone:      "0922222222",
		DriverID:           driver.ID,
	}, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("failed to create shipment: %v", err)
	}
	if shipment.Status != "assigned" {
		t.Errorf("expected status assigned when driver provided, got %s", shipment.Status)
	}
	if shipment.DriverID != driver.ID {
		t.Errorf("expected driver ID %s, got %s", driver.ID, shipment.DriverID)
	}
}
func TestShipmentListing_Pagination(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()
	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "shipment-list")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "ship-list@test.com")
	shipmentService := env.GetShipmentService()
	for i := 0; i < 30; i++ {
		_, err := shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID, service.CreateShipmentInput{
			OriginAddress:      "Addis Ababa",
			DestinationAddress: fmt.Sprintf("City %d", i),
			CustomerName:       fmt.Sprintf("Customer %d", i),
			CustomerPhone:      "0911111111",
		}, "127.0.0.1", "test-agent")
		if err != nil {
			t.Fatalf("failed to create shipment %d: %v", i, err)
		}
	}
	shipments, total, err := shipmentService.ListByTenant(ctx, tenant.ID.String(), 1, 10)
	if err != nil {
		t.Fatalf("failed to list shipments: %v", err)
	}
	if len(shipments) != 10 {
		t.Errorf("expected 10 shipments, got %d", len(shipments))
	}
	if total != 30 {
		t.Errorf("expected total 30, got %d", total)
	}
	page2, _, err := shipmentService.ListByTenant(ctx, tenant.ID.String(), 2, 10)
	if err != nil {
		t.Fatalf("failed to list page 2: %v", err)
	}
	if len(page2) != 10 {
		t.Errorf("expected 10 shipments on page 2, got %d", len(page2))
	}
}
func TestShipmentGetByID_Success(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()
	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "shipment-get")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "ship-get@test.com")
	shipmentService := env.GetShipmentService()
	created, err := shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID, service.CreateShipmentInput{
		OriginAddress:      "Addis Ababa",
		DestinationAddress: "Hawassa",
		CustomerName:       "Test Customer",
		CustomerPhone:      "0933333333",
	}, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("failed to create shipment: %v", err)
	}
	fetched, err := shipmentService.GetByID(ctx, tenant.ID.String(), created.ID)
	if err != nil {
		t.Fatalf("failed to get shipment: %v", err)
	}
	if fetched.ID != created.ID {
		t.Errorf("expected ID %s, got %s", created.ID, fetched.ID)
	}
	if fetched.TrackingNumber != created.TrackingNumber {
		t.Errorf("expected tracking number %s, got %s", created.TrackingNumber, fetched.TrackingNumber)
	}
}
func TestShipmentGetByID_NotFound(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()
	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "shipment-notfound")
	shipmentService := env.GetShipmentService()
	_, err := shipmentService.GetByID(ctx, tenant.ID.String(), "00000000-0000-0000-0000-000000000000")
	if err == nil {
		t.Error("expected error for non-existent shipment")
	}
}
func TestShipmentGetByTrackingNumber(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()
	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "shipment-tracking")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "ship-track@test.com")
	shipmentService := env.GetShipmentService()
	created, err := shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID, service.CreateShipmentInput{
		OriginAddress:      "Addis Ababa",
		DestinationAddress: "Mekelle",
		CustomerName:       "Test Customer",
		CustomerPhone:      "0944444444",
	}, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("failed to create shipment: %v", err)
	}
	fetched, err := shipmentService.GetByTrackingNumber(ctx, created.TrackingNumber)
	if err != nil {
		t.Fatalf("failed to get shipment by tracking number: %v", err)
	}
	if fetched.ID != created.ID {
		t.Errorf("expected ID %s, got %s", created.ID, fetched.ID)
	}
}
func TestShipmentAssignDriver_Success(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()
	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "shipment-assign")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "ship-assign@test.com")
	driver := env.CreateTestDriver(ctx, tenant.ID.String(), "0955555555")
	shipmentService := env.GetShipmentService()
	shipment, err := shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID, service.CreateShipmentInput{
		OriginAddress:      "Addis Ababa",
		DestinationAddress: "Gondar",
		CustomerName:       "Test Customer",
		CustomerPhone:      "0955555555",
	}, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("failed to create shipment: %v", err)
	}
	assigned, err := shipmentService.AssignDriver(ctx, tenant.ID.String(), dispatcher.ID, shipment.ID, driver.ID, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("failed to assign driver: %v", err)
	}
	if assigned.Status != "assigned" {
		t.Errorf("expected status assigned, got %s", assigned.Status)
	}
	if assigned.DriverID != driver.ID {
		t.Errorf("expected driver ID %s, got %s", driver.ID, assigned.DriverID)
	}
}
func TestShipmentAssignDriver_AlreadyAssigned(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()
	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "shipment-reassign")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "ship-reassign@test.com")
	driver1 := env.CreateTestDriver(ctx, tenant.ID.String(), "0966666661")
	driver2 := env.CreateTestDriver(ctx, tenant.ID.String(), "0966666662")
	shipmentService := env.GetShipmentService()
	shipment, err := shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID, service.CreateShipmentInput{
		OriginAddress:      "Addis Ababa",
		DestinationAddress: "Jimma",
		CustomerName:       "Test Customer",
		CustomerPhone:      "0966666666",
	}, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("failed to create shipment: %v", err)
	}
	_, err = shipmentService.AssignDriver(ctx, tenant.ID.String(), dispatcher.ID, shipment.ID, driver1.ID, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("failed to assign driver: %v", err)
	}
	reassigned, err := shipmentService.AssignDriver(ctx, tenant.ID.String(), dispatcher.ID, shipment.ID, driver2.ID, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("failed to reassign driver: %v", err)
	}
	if reassigned.DriverID != driver2.ID {
		t.Errorf("expected driver ID %s, got %s", driver2.ID, reassigned.DriverID)
	}
}
func TestShipmentUpdateStatus_Success(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()
	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "shipment-status")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "ship-status@test.com")
	driver := env.CreateTestDriver(ctx, tenant.ID.String(), "0977777777")
	shipmentService := env.GetShipmentService()
	shipment, err := shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID, service.CreateShipmentInput{
		OriginAddress:      "Addis Ababa",
		DestinationAddress: "Dessie",
		CustomerName:       "Test Customer",
		CustomerPhone:      "0977777777",
		DriverID:           driver.ID,
	}, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("failed to create shipment: %v", err)
	}
	inTransit, err := shipmentService.UpdateStatus(ctx, tenant.ID.String(), dispatcher.ID, "dispatcher", shipment.ID, service.UpdateStatusInput{
		Status:     "in_transit",
		StatusNote: "Driver started delivery",
	}, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("failed to update status: %v", err)
	}
	if inTransit.Status != "in_transit" {
		t.Errorf("expected status in_transit, got %s", inTransit.Status)
	}
	if inTransit.StatusNote != "Driver started delivery" {
		t.Errorf("expected status note, got %s", inTransit.StatusNote)
	}
}
func TestShipmentUpdateStatus_Delivered(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()
	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "shipment-delivered")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "ship-delivered@test.com")
	driver := env.CreateTestDriver(ctx, tenant.ID.String(), "0988888888")
	shipmentService := env.GetShipmentService()
	shipment, err := shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID, service.CreateShipmentInput{
		OriginAddress:      "Addis Ababa",
		DestinationAddress: "Adama",
		CustomerName:       "Test Customer",
		CustomerPhone:      "0988888888",
		DriverID:           driver.ID,
	}, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("failed to create shipment: %v", err)
	}
	_, _ = shipmentService.UpdateStatus(ctx, tenant.ID.String(), dispatcher.ID, "dispatcher", shipment.ID, service.UpdateStatusInput{
		Status: "in_transit",
	}, "127.0.0.1", "test-agent")
	delivered, err := shipmentService.UpdateStatus(ctx, tenant.ID.String(), dispatcher.ID, "dispatcher", shipment.ID, service.UpdateStatusInput{
		Status:     "delivered",
		StatusNote: "Package delivered successfully",
	}, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("failed to mark as delivered: %v", err)
	}
	if delivered.Status != "delivered" {
		t.Errorf("expected status delivered, got %s", delivered.Status)
	}
	if delivered.ActualDelivery == nil {
		t.Error("expected actual delivery time to be set")
	}
}
func TestShipmentCancellation_Success(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()
	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "shipment-cancel")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "ship-cancel@test.com")
	shipmentService := env.GetShipmentService()
	shipment, err := shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID, service.CreateShipmentInput{
		OriginAddress:      "Addis Ababa",
		DestinationAddress: "Harar",
		CustomerName:       "Test Customer",
		CustomerPhone:      "0999999999",
	}, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("failed to create shipment: %v", err)
	}
	cancelled, err := shipmentService.Cancel(ctx, tenant.ID.String(), dispatcher.ID, shipment.ID, service.CancelShipmentInput{
		Reason: "Customer requested cancellation",
	}, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("failed to cancel shipment: %v", err)
	}
	if cancelled.Status != "cancelled" {
		t.Errorf("expected status cancelled, got %s", cancelled.Status)
	}
	if cancelled.StatusReason != "Customer requested cancellation" {
		t.Errorf("expected cancellation reason, got %s", cancelled.StatusReason)
	}
}
func TestShipmentCancellation_AlreadyCancelled(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()
	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "shipment-cancel-twice")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "ship-cancel2@test.com")
	shipmentService := env.GetShipmentService()
	shipment, err := shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID, service.CreateShipmentInput{
		OriginAddress:      "Addis Ababa",
		DestinationAddress: "Harar",
		CustomerName:       "Test Customer",
		CustomerPhone:      "0999999991",
	}, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("failed to create shipment: %v", err)
	}
	_, err = shipmentService.Cancel(ctx, tenant.ID.String(), dispatcher.ID, shipment.ID, service.CancelShipmentInput{
		Reason: "Customer requested cancellation",
	}, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("failed to cancel shipment: %v", err)
	}
	_, err = shipmentService.Cancel(ctx, tenant.ID.String(), dispatcher.ID, shipment.ID, service.CancelShipmentInput{
		Reason: "Trying again",
	}, "127.0.0.1", "test-agent")
	if err == nil {
		t.Error("expected error when cancelling already cancelled shipment")
	}
}
func TestShipmentCancellation_CannotCancelDelivered(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()
	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "shipment-cancel-delivered")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "ship-cancel3@test.com")
	driver := env.CreateTestDriver(ctx, tenant.ID.String(), "0911111122")
	shipmentService := env.GetShipmentService()
	shipment, err := shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID, service.CreateShipmentInput{
		OriginAddress:      "Addis Ababa",
		DestinationAddress: "Adama",
		CustomerName:       "Test Customer",
		CustomerPhone:      "0911111133",
		DriverID:           driver.ID,
	}, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("failed to create shipment: %v", err)
	}
	_, _ = shipmentService.UpdateStatus(ctx, tenant.ID.String(), dispatcher.ID, "dispatcher", shipment.ID, service.UpdateStatusInput{
		Status: "in_transit",
	}, "127.0.0.1", "test-agent")
	_, _ = shipmentService.UpdateStatus(ctx, tenant.ID.String(), dispatcher.ID, "dispatcher", shipment.ID, service.UpdateStatusInput{
		Status: "delivered",
	}, "127.0.0.1", "test-agent")
	_, err = shipmentService.Cancel(ctx, tenant.ID.String(), dispatcher.ID, shipment.ID, service.CancelShipmentInput{
		Reason: "Trying to cancel",
	}, "127.0.0.1", "test-agent")
	if err == nil {
		t.Error("expected error when cancelling delivered shipment")
	}
}
func TestShipmentSearch_ByStatus(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()
	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "shipment-search-status")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "ship-search@test.com")
	driver := env.CreateTestDriver(ctx, tenant.ID.String(), "0922222222")
	shipmentService := env.GetShipmentService()
	for i := 0; i < 5; i++ {
		_, _ = shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID, service.CreateShipmentInput{
			OriginAddress:      "Addis Ababa",
			DestinationAddress: "City A",
			CustomerName:       "Customer",
			CustomerPhone:      "0911111111",
		}, "127.0.0.1", "test-agent")
	}
	for i := 0; i < 3; i++ {
		shipment, _ := shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID, service.CreateShipmentInput{
			OriginAddress:      "Addis Ababa",
			DestinationAddress: "City B",
			CustomerName:       "Customer",
			CustomerPhone:      "0922222222",
			DriverID:           driver.ID,
		}, "127.0.0.1", "test-agent")
		_, _ = shipmentService.UpdateStatus(ctx, tenant.ID.String(), dispatcher.ID, "dispatcher", shipment.ID, service.UpdateStatusInput{
			Status: "in_transit",
		}, "127.0.0.1", "test-agent")
	}
	shipments, _, err := shipmentService.Search(ctx, tenant.ID.String(), service.SearchShipmentsInput{
		Status:  "pending",
		Page:    1,
		PerPage: 10,
	})
	if err != nil {
		t.Fatalf("failed to search shipments: %v", err)
	}
	if len(shipments) != 5 {
		t.Errorf("expected 5 pending shipments, got %d", len(shipments))
	}
	inTransit, _, err := shipmentService.Search(ctx, tenant.ID.String(), service.SearchShipmentsInput{
		Status:  "in_transit",
		Page:    1,
		PerPage: 10,
	})
	if err != nil {
		t.Fatalf("failed to search in_transit: %v", err)
	}
	if len(inTransit) != 3 {
		t.Errorf("expected 3 in_transit shipments, got %d", len(inTransit))
	}
}
func TestShipmentSearch_ByQuery(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()
	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "shipment-search-query")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "ship-search-q@test.com")
	shipmentService := env.GetShipmentService()
	shipment1, _ := shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID, service.CreateShipmentInput{
		OriginAddress:      "Addis Ababa",
		DestinationAddress: "City A",
		CustomerName:       "John Doe",
		CustomerPhone:      "0911111111",
	}, "127.0.0.1", "test-agent")
	_, _ = shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID, service.CreateShipmentInput{
		OriginAddress:      "Addis Ababa",
		DestinationAddress: "City B",
		CustomerName:       "Jane Smith",
		CustomerPhone:      "0922222222",
	}, "127.0.0.1", "test-agent")
	results, _, err := shipmentService.Search(ctx, tenant.ID.String(), service.SearchShipmentsInput{
		Query:   "John",
		Page:    1,
		PerPage: 10,
	})
	if err != nil {
		t.Fatalf("failed to search by customer name: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'John', got %d", len(results))
	}
	if len(results) > 0 && results[0].ID != shipment1.ID {
		t.Errorf("expected shipment %s, got %s", shipment1.ID, results[0].ID)
	}
}
func TestShipmentSearch_ByTrackingNumber(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()
	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "shipment-search-track")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "ship-search-track@test.com")
	shipmentService := env.GetShipmentService()
	shipment, _ := shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID, service.CreateShipmentInput{
		OriginAddress:      "Addis Ababa",
		DestinationAddress: "City A",
		CustomerName:       "Customer",
		CustomerPhone:      "0911111111",
	}, "127.0.0.1", "test-agent")
	results, _, err := shipmentService.Search(ctx, tenant.ID.String(), service.SearchShipmentsInput{
		Query:   shipment.TrackingNumber,
		Page:    1,
		PerPage: 10,
	})
	if err != nil {
		t.Fatalf("failed to search by tracking number: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result for tracking number, got %d", len(results))
	}
}
func TestShipmentTenantIsolation(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()
	ctx := context.Background()
	tenant1 := env.CreateTestTenant(ctx, "tenant1-iso")
	tenant2 := env.CreateTestTenant(ctx, "tenant2-iso")
	dispatcher1 := env.CreateTestDispatcher(ctx, tenant1.ID.String(), "disp1@iso.com")
	dispatcher2 := env.CreateTestDispatcher(ctx, tenant2.ID.String(), "disp2@iso.com")
	shipmentService := env.GetShipmentService()
	shipment1, _ := shipmentService.Create(ctx, tenant1.ID.String(), dispatcher1.ID, service.CreateShipmentInput{
		OriginAddress:      "City A",
		DestinationAddress: "City B",
		CustomerName:       "Tenant1 Customer",
		CustomerPhone:      "0911111111",
	}, "127.0.0.1", "test-agent")
	shipment2, _ := shipmentService.Create(ctx, tenant2.ID.String(), dispatcher2.ID, service.CreateShipmentInput{
		OriginAddress:      "City C",
		DestinationAddress: "City D",
		CustomerName:       "Tenant2 Customer",
		CustomerPhone:      "0922222222",
	}, "127.0.0.1", "test-agent")
	_, err := shipmentService.GetByID(ctx, tenant1.ID.String(), shipment2.ID)
	if err == nil {
		t.Error("expected error when tenant1 tries to access tenant2's shipment")
	}
	_, err = shipmentService.GetByID(ctx, tenant2.ID.String(), shipment1.ID)
	if err == nil {
		t.Error("expected error when tenant2 tries to access tenant1's shipment")
	}
	_, err = shipmentService.AssignDriver(ctx, tenant1.ID.String(), dispatcher1.ID, shipment2.ID, "some-driver-id", "127.0.0.1", "test-agent")
	if err == nil {
		t.Error("expected error when tenant1 tries to modify tenant2's shipment")
	}
	_, err = shipmentService.Cancel(ctx, tenant2.ID.String(), dispatcher2.ID, shipment1.ID, service.CancelShipmentInput{
		Reason: "Cross-tenant cancel attempt",
	}, "127.0.0.1", "test-agent")
	if err == nil {
		t.Error("expected error when tenant2 tries to cancel tenant1's shipment")
	}
}
func TestShipmentListing_TenantIsolation(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()
	ctx := context.Background()
	tenant1 := env.CreateTestTenant(ctx, "tenant1-list")
	tenant2 := env.CreateTestTenant(ctx, "tenant2-list")
	dispatcher1 := env.CreateTestDispatcher(ctx, tenant1.ID.String(), "disp1@list.com")
	dispatcher2 := env.CreateTestDispatcher(ctx, tenant2.ID.String(), "disp2@list.com")
	shipmentService := env.GetShipmentService()
	for i := 0; i < 5; i++ {
		_, _ = shipmentService.Create(ctx, tenant1.ID.String(), dispatcher1.ID, service.CreateShipmentInput{
			OriginAddress:      "City A",
			DestinationAddress: "City B",
			CustomerName:       "Tenant1 Customer",
			CustomerPhone:      "0911111111",
		}, "127.0.0.1", "test-agent")
	}
	for i := 0; i < 3; i++ {
		_, _ = shipmentService.Create(ctx, tenant2.ID.String(), dispatcher2.ID, service.CreateShipmentInput{
			OriginAddress:      "City C",
			DestinationAddress: "City D",
			CustomerName:       "Tenant2 Customer",
			CustomerPhone:      "0922222222",
		}, "127.0.0.1", "test-agent")
	}
	list1, total1, _ := shipmentService.ListByTenant(ctx, tenant1.ID.String(), 1, 100)
	list2, total2, _ := shipmentService.ListByTenant(ctx, tenant2.ID.String(), 1, 100)
	if total1 != 5 {
		t.Errorf("expected tenant1 to have 5 shipments, got %d", total1)
	}
	if total2 != 3 {
		t.Errorf("expected tenant2 to have 3 shipments, got %d", total2)
	}
	for _, s := range list1 {
		if s.TenantID != tenant1.ID.String() {
			t.Errorf("tenant1 list contains shipment from different tenant: %s", s.TenantID)
		}
	}
	for _, s := range list2 {
		if s.TenantID != tenant2.ID.String() {
			t.Errorf("tenant2 list contains shipment from different tenant: %s", s.TenantID)
		}
	}
}
func TestListActiveShipmentsByDriver(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()
	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "driver-shipments")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "disp@driver.com")
	driver := env.CreateTestDriver(ctx, tenant.ID.String(), "0933333333")
	shipmentService := env.GetShipmentService()
	_, _ = shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID, service.CreateShipmentInput{
		OriginAddress:      "City A",
		DestinationAddress: "City B",
		CustomerName:       "Customer",
		CustomerPhone:      "0911111111",
	}, "127.0.0.1", "test-agent")
	_, _ = shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID, service.CreateShipmentInput{
		OriginAddress:      "City C",
		DestinationAddress: "City D",
		CustomerName:       "Customer",
		CustomerPhone:      "0922222222",
		DriverID:           driver.ID,
	}, "127.0.0.1", "test-agent")
	assigned2, _ := shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID, service.CreateShipmentInput{
		OriginAddress:      "City E",
		DestinationAddress: "City F",
		CustomerName:       "Customer",
		CustomerPhone:      "0933333333",
		DriverID:           driver.ID,
	}, "127.0.0.1", "test-agent")
	_, _ = shipmentService.UpdateStatus(ctx, tenant.ID.String(), dispatcher.ID, "dispatcher", assigned2.ID, service.UpdateStatusInput{
		Status: "in_transit",
	}, "127.0.0.1", "test-agent")
	delivered, _ := shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID, service.CreateShipmentInput{
		OriginAddress:      "City G",
		DestinationAddress: "City H",
		CustomerName:       "Customer",
		CustomerPhone:      "0944444444",
		DriverID:           driver.ID,
	}, "127.0.0.1", "test-agent")
	_, _ = shipmentService.UpdateStatus(ctx, tenant.ID.String(), dispatcher.ID, "dispatcher", delivered.ID, service.UpdateStatusInput{
		Status: "delivered",
	}, "127.0.0.1", "test-agent")
	activeShipments, err := shipmentService.ListActiveByDriver(ctx, tenant.ID.String(), driver.ID)
	if err != nil {
		t.Fatalf("failed to list active shipments: %v", err)
	}
	for _, s := range activeShipments {
		if s.Status == "delivered" || s.Status == "cancelled" {
			t.Errorf("unexpected non-active shipment with status: %s", s.Status)
		}
	}
	if len(activeShipments) < 2 {
		t.Errorf("expected at least 2 active shipments, got %d", len(activeShipments))
	}
}
func TestShipmentUpdate_Success(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()
	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "shipment-update")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "ship-update@test.com")
	shipmentService := env.GetShipmentService()
	shipment, _ := shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID, service.CreateShipmentInput{
		OriginAddress:      "Addis Ababa",
		DestinationAddress: "Dire Dawa",
		CustomerName:       "Original Name",
		CustomerPhone:      "0911111111",
	}, "127.0.0.1", "test-agent")
	updated, err := shipmentService.Update(ctx, tenant.ID.String(), dispatcher.ID, shipment.ID, service.UpdateShipmentInput{
		CustomerName:  "Updated Name",
		CustomerPhone: "0922222222",
	}, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("failed to update shipment: %v", err)
	}
	if updated.CustomerName != "Updated Name" {
		t.Errorf("expected customer name 'Updated Name', got %s", updated.CustomerName)
	}
	if updated.CustomerPhone != "0922222222" {
		t.Errorf("expected customer phone '0922222222', got %s", updated.CustomerPhone)
	}
}
func TestShipmentCreation_HTTP(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()
	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "shipment-http")
	_ = env.CreateTestDispatcher(ctx, tenant.ID.String(), "ship-http@test.com")
	authService := env.GetAuthService()
	loginResult, _ := authService.DispatcherLogin(ctx, service.DispatcherLoginInput{
		Email:    "ship-http@test.com",
		Password: "password123",
	}, "127.0.0.1", "test-agent")
	body := `{
		"origin_address": "Addis Ababa",
		"destination_address": "Dire Dawa",
		"customer_name": "HTTP Customer",
		"customer_phone": "0911111111",
		"cargo_description": "Test cargo"
	}`
	req := httptest.NewRequest(http.MethodPost, apiPrefix+"/shipments", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+loginResult.AccessToken)
	rec := httptest.NewRecorder()
	env.GetRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}
}
func TestShipmentGet_HTTP(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()
	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "shipment-get-http")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "ship-get-http@test.com")
	shipmentService := env.GetShipmentService()
	shipment, _ := shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID, service.CreateShipmentInput{
		OriginAddress:      "Addis Ababa",
		DestinationAddress: "Dire Dawa",
		CustomerName:       "Test Customer",
		CustomerPhone:      "0911111111",
	}, "127.0.0.1", "test-agent")
	authService := env.GetAuthService()
	loginResult, _ := authService.DispatcherLogin(ctx, service.DispatcherLoginInput{
		Email:    "ship-get-http@test.com",
		Password: "password123",
	}, "127.0.0.1", "test-agent")
	req := httptest.NewRequest(http.MethodGet, apiPrefix+"/shipments/"+shipment.ID, nil)
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
	if data["id"] != shipment.ID {
		t.Errorf("expected shipment ID %s, got %v", shipment.ID, data["id"])
	}
}
func TestShipmentCancel_HTTP(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()
	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "shipment-cancel-http")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "ship-cancel-http@test.com")
	shipmentService := env.GetShipmentService()
	shipment, _ := shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID, service.CreateShipmentInput{
		OriginAddress:      "Addis Ababa",
		DestinationAddress: "Dire Dawa",
		CustomerName:       "Test Customer",
		CustomerPhone:      "0911111111",
	}, "127.0.0.1", "test-agent")
	authService := env.GetAuthService()
	loginResult, _ := authService.DispatcherLogin(ctx, service.DispatcherLoginInput{
		Email:    "ship-cancel-http@test.com",
		Password: "password123",
	}, "127.0.0.1", "test-agent")
	body := `{"reason": "Customer requested cancellation"}`
	req := httptest.NewRequest(http.MethodPost, apiPrefix+"/shipments/"+shipment.ID+"/cancel", strings.NewReader(body))
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
	if data["status"] != "cancelled" {
		t.Errorf("expected status cancelled, got %v", data["status"])
	}
}
func TestTrackShipment_HTTP_Public(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()
	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "track-public")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "track-public@test.com")
	shipmentService := env.GetShipmentService()
	shipment, _ := shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID, service.CreateShipmentInput{
		OriginAddress:      "Addis Ababa",
		DestinationAddress: "Dire Dawa",
		CustomerName:       "Test Customer",
		CustomerPhone:      "0911111111",
	}, "127.0.0.1", "test-agent")
	req := httptest.NewRequest(http.MethodGet, apiPrefix+"/track/"+shipment.TrackingNumber, nil)
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
	if data["tracking_number"] != shipment.TrackingNumber {
		t.Errorf("expected tracking number %s, got %v", shipment.TrackingNumber, data["tracking_number"])
	}
}
func TestShipmentSearch_HTTP(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()
	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "shipment-search-http")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "ship-search-http@test.com")
	shipmentService := env.GetShipmentService()
	for i := 0; i < 5; i++ {
		_, _ = shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID, service.CreateShipmentInput{
			OriginAddress:      "Addis Ababa",
			DestinationAddress: "Dire Dawa",
			CustomerName:       fmt.Sprintf("Customer %d", i),
			CustomerPhone:      "0911111111",
		}, "127.0.0.1", "test-agent")
	}
	authService := env.GetAuthService()
	loginResult, _ := authService.DispatcherLogin(ctx, service.DispatcherLoginInput{
		Email:    "ship-search-http@test.com",
		Password: "password123",
	}, "127.0.0.1", "test-agent")
	req := httptest.NewRequest(http.MethodGet, apiPrefix+"/shipments/search?status=pending&page=1&per_page=10", nil)
	req.Header.Set("Authorization", "Bearer "+loginResult.AccessToken)
	rec := httptest.NewRecorder()
	env.GetRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
}
