package sms

import (
	"context"
	"testing"
	"time"
)

func TestInMemoryAuditLogger_LogSMS(t *testing.T) {
	logger := NewInMemoryAuditLogger()

	entry := AuditEntry{
		ID:          "msg-123",
		Phone:       "+251912345678",
		Message:     "Test message",
		MessageType: MessageTypeNotification,
		Status:      "sent",
		Provider:    "mock",
	}

	err := logger.LogSMS(context.Background(), entry)
	if err != nil {
		t.Fatalf("LogSMS() error = %v", err)
	}

	entries := logger.GetEntries()
	if len(entries) != 1 {
		t.Fatalf("GetEntries() returned %d entries, want 1", len(entries))
	}

	if entries[0].ID != entry.ID {
		t.Errorf("entry.ID = %q, want %q", entries[0].ID, entry.ID)
	}
	if entries[0].Phone != entry.Phone {
		t.Errorf("entry.Phone = %q, want %q", entries[0].Phone, entry.Phone)
	}
	if entries[0].CreatedAt.IsZero() {
		t.Error("entry.CreatedAt should be set")
	}
}

func TestInMemoryAuditLogger_GetEntries(t *testing.T) {
	logger := NewInMemoryAuditLogger()

	entries := []AuditEntry{
		{
			ID:          "msg-1",
			Phone:       "+251912345678",
			Message:     "Message 1",
			MessageType: MessageTypeNotification,
			Status:      "sent",
			Provider:    "mock",
		},
		{
			ID:          "msg-2",
			Phone:       "+251923456789",
			Message:     "Message 2",
			MessageType: MessageTypeOTP,
			Status:      "sent",
			Provider:    "twilio",
		},
		{
			ID:          "msg-3",
			Phone:       "+251934567890",
			Message:     "Message 3",
			MessageType: MessageTypeAlert,
			Status:      "failed",
			Provider:    "ethiotelecom",
		},
	}

	for _, e := range entries {
		logger.LogSMS(context.Background(), e)
	}

	result := logger.GetEntries()
	if len(result) != len(entries) {
		t.Fatalf("GetEntries() returned %d entries, want %d", len(result), len(entries))
	}
}

func TestInMemoryAuditLogger_GetEntriesByPhone(t *testing.T) {
	logger := NewInMemoryAuditLogger()

	phone1 := "+251912345678"
	phone2 := "+251923456789"

	entries := []AuditEntry{
		{
			ID:          "msg-1",
			Phone:       phone1,
			Message:     "Message 1",
			MessageType: MessageTypeNotification,
			Status:      "sent",
			Provider:    "mock",
		},
		{
			ID:          "msg-2",
			Phone:       phone2,
			Message:     "Message 2",
			MessageType: MessageTypeOTP,
			Status:      "sent",
			Provider:    "twilio",
		},
		{
			ID:          "msg-3",
			Phone:       phone1,
			Message:     "Message 3",
			MessageType: MessageTypeAlert,
			Status:      "sent",
			Provider:    "mock",
		},
	}

	for _, e := range entries {
		logger.LogSMS(context.Background(), e)
	}

	result := logger.GetEntriesByPhone(phone1)
	if len(result) != 2 {
		t.Fatalf("GetEntriesByPhone() returned %d entries, want 2", len(result))
	}

	for _, e := range result {
		if e.Phone != phone1 {
			t.Errorf("GetEntriesByPhone() returned entry with phone %q, want %q", e.Phone, phone1)
		}
	}

	result = logger.GetEntriesByPhone(phone2)
	if len(result) != 1 {
		t.Fatalf("GetEntriesByPhone() returned %d entries, want 1", len(result))
	}

	result = logger.GetEntriesByPhone("+251999999999")
	if len(result) != 0 {
		t.Errorf("GetEntriesByPhone() should return empty slice for unknown phone, got %d entries", len(result))
	}
}

func TestInMemoryAuditLogger_Clear(t *testing.T) {
	logger := NewInMemoryAuditLogger()

	entry := AuditEntry{
		ID:          "msg-123",
		Phone:       "+251912345678",
		Message:     "Test message",
		MessageType: MessageTypeNotification,
		Status:      "sent",
		Provider:    "mock",
	}

	logger.LogSMS(context.Background(), entry)

	if len(logger.GetEntries()) != 1 {
		t.Fatal("LogSMS() failed to add entry")
	}

	logger.Clear()

	entries := logger.GetEntries()
	if len(entries) != 0 {
		t.Errorf("Clear() should remove all entries, got %d entries", len(entries))
	}
}

func TestInMemoryAuditLogger_EntryFields(t *testing.T) {
	logger := NewInMemoryAuditLogger()

	now := time.Now()
	entry := AuditEntry{
		ID:            "msg-123",
		Phone:         "+251912345678",
		Message:       "Test message",
		MessageType:   MessageTypeNotification,
		Status:        "sent",
		Provider:      "mock",
		Error:         "",
		RetryCount:    2,
		Duration:      150 * time.Millisecond,
		TenantID:      "tenant-1",
		ShipmentID:    "ship-1",
		CorrelationID: "corr-1",
	}

	logger.LogSMS(context.Background(), entry)

	entries := logger.GetEntries()
	if len(entries) != 1 {
		t.Fatal("expected one entry")
	}

	saved := entries[0]
	if saved.ID != entry.ID {
		t.Errorf("ID = %q, want %q", saved.ID, entry.ID)
	}
	if saved.Phone != entry.Phone {
		t.Errorf("Phone = %q, want %q", saved.Phone, entry.Phone)
	}
	if saved.Message != entry.Message {
		t.Errorf("Message = %q, want %q", saved.Message, entry.Message)
	}
	if saved.MessageType != entry.MessageType {
		t.Errorf("MessageType = %q, want %q", saved.MessageType, entry.MessageType)
	}
	if saved.Status != entry.Status {
		t.Errorf("Status = %q, want %q", saved.Status, entry.Status)
	}
	if saved.Provider != entry.Provider {
		t.Errorf("Provider = %q, want %q", saved.Provider, entry.Provider)
	}
	if saved.RetryCount != entry.RetryCount {
		t.Errorf("RetryCount = %d, want %d", saved.RetryCount, entry.RetryCount)
	}
	if saved.Duration != entry.Duration {
		t.Errorf("Duration = %v, want %v", saved.Duration, entry.Duration)
	}
	if saved.TenantID != entry.TenantID {
		t.Errorf("TenantID = %q, want %q", saved.TenantID, entry.TenantID)
	}
	if saved.ShipmentID != entry.ShipmentID {
		t.Errorf("ShipmentID = %q, want %q", saved.ShipmentID, entry.ShipmentID)
	}
	if saved.CorrelationID != entry.CorrelationID {
		t.Errorf("CorrelationID = %q, want %q", saved.CorrelationID, entry.CorrelationID)
	}
	if saved.CreatedAt.Before(now) {
		t.Error("CreatedAt should be set to current time")
	}
}
