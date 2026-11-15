package sms

import (
	"context"
	"fmt"
	"testing"
)

func TestMockProvider_Send_StoresMessage(t *testing.T) {
	logger := &mockLogger{}
	provider := NewMockProvider(logger)

	_, err := provider.Send(context.Background(), "+251912345678", "Test message", MessageTypeNotification)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	messages := provider.GetMessages()
	if len(messages) != 1 {
		t.Fatalf("GetMessages() returned %d messages, want 1", len(messages))
	}

	if messages[0].Message != "Test message" {
		t.Errorf("message.Message = %q, want %q", messages[0].Message, "Test message")
	}
	if messages[0].Type != MessageTypeNotification {
		t.Errorf("message.Type = %q, want %q", messages[0].Type, MessageTypeNotification)
	}
	if messages[0].Status != "sent" {
		t.Errorf("message.Status = %q, want %q", messages[0].Status, "sent")
	}
}

func TestMockProvider_Send_NormalizesPhone(t *testing.T) {
	logger := &mockLogger{}
	provider := NewMockProvider(logger)

	_, err := provider.Send(context.Background(), "0912345678", "Test message", MessageTypeOTP)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	messages := provider.GetMessages()
	if len(messages) != 1 {
		t.Fatal("expected one message")
	}

	if messages[0].To != "+251912345678" {
		t.Errorf("message.To = %q, want %q", messages[0].To, "+251912345678")
	}
}

func TestMockProvider_Send_InvalidPhone(t *testing.T) {
	logger := &mockLogger{}
	provider := NewMockProvider(logger)

	_, err := provider.Send(context.Background(), "invalid", "Test message", MessageTypeNotification)
	if err == nil {
		t.Error("Send() should return error for invalid phone number")
	}
}

func TestMockProvider_GetMessages(t *testing.T) {
	logger := &mockLogger{}
	provider := NewMockProvider(logger)

	send(t, provider, "+251912345678", "Message 1", MessageTypeNotification)
	send(t, provider, "+251923456789", "Message 2", MessageTypeOTP)

	messages := provider.GetMessages()
	if len(messages) != 2 {
		t.Fatalf("GetMessages() returned %d messages, want 2", len(messages))
	}
}

func TestMockProvider_GetMessagesByPhone(t *testing.T) {
	logger := &mockLogger{}
	provider := NewMockProvider(logger)

	phone1 := "+251912345678"
	phone2 := "+251923456789"

	send(t, provider, phone1, "Message 1", MessageTypeNotification)
	send(t, provider, phone2, "Message 2", MessageTypeOTP)
	send(t, provider, phone1, "Message 3", MessageTypeAlert)

	messages := provider.GetMessagesByPhone(phone1)
	if len(messages) != 2 {
		t.Fatalf("GetMessagesByPhone() returned %d messages, want 2", len(messages))
	}

	for _, m := range messages {
		if m.To != phone1 {
			t.Errorf("message.To = %q, want %q", m.To, phone1)
		}
	}
}

func TestMockProvider_GetMessagesByPhone_LocalFormat(t *testing.T) {
	logger := &mockLogger{}
	provider := NewMockProvider(logger)

	send(t, provider, "0912345678", "Test", MessageTypeNotification)

	messages := provider.GetMessagesByPhone("0912345678")
	if len(messages) != 1 {
		t.Fatalf("GetMessagesByPhone() returned %d messages, want 1", len(messages))
	}
}

func TestMockProvider_GetMessagesByPhone_InvalidPhone(t *testing.T) {
	logger := &mockLogger{}
	provider := NewMockProvider(logger)

	send(t, provider, "+251912345678", "Test", MessageTypeNotification)

	messages := provider.GetMessagesByPhone("invalid")
	if messages != nil {
		t.Errorf("GetMessagesByPhone() with invalid phone should return nil, got %d messages", len(messages))
	}
}

func TestMockProvider_GetMessagesByPhone_NoMatch(t *testing.T) {
	logger := &mockLogger{}
	provider := NewMockProvider(logger)

	send(t, provider, "+251912345678", "Test", MessageTypeNotification)

	messages := provider.GetMessagesByPhone("+251999999999")
	if len(messages) != 0 {
		t.Errorf("GetMessagesByPhone() should return empty slice for no match, got %d messages", len(messages))
	}
}

func TestMockProvider_GetLastMessage(t *testing.T) {
	logger := &mockLogger{}
	provider := NewMockProvider(logger)

	send(t, provider, "+251912345678", "Message 1", MessageTypeNotification)
	send(t, provider, "+251923456789", "Message 2", MessageTypeOTP)

	lastMsg := provider.GetLastMessage()
	if lastMsg == nil {
		t.Fatal("GetLastMessage() returned nil")
	}

	if lastMsg.Message != "Message 2" {
		t.Errorf("lastMsg.Message = %q, want %q", lastMsg.Message, "Message 2")
	}
}

func TestMockProvider_GetLastMessage_Empty(t *testing.T) {
	logger := &mockLogger{}
	provider := NewMockProvider(logger)

	lastMsg := provider.GetLastMessage()
	if lastMsg != nil {
		t.Error("GetLastMessage() should return nil when no messages")
	}
}

func TestMockProvider_Clear(t *testing.T) {
	logger := &mockLogger{}
	provider := NewMockProvider(logger)

	send(t, provider, "+251912345678", "Test", MessageTypeNotification)

	if provider.Count() != 1 {
		t.Fatal("expected one message before clear")
	}

	provider.Clear()

	if provider.Count() != 0 {
		t.Errorf("Count() = %d, want 0 after clear", provider.Count())
	}
}

func TestMockProvider_Count(t *testing.T) {
	logger := &mockLogger{}
	provider := NewMockProvider(logger)

	if provider.Count() != 0 {
		t.Errorf("Count() = %d, want 0 for new provider", provider.Count())
	}

	send(t, provider, "+251912345678", "Test 1", MessageTypeNotification)
	if provider.Count() != 1 {
		t.Errorf("Count() = %d, want 1", provider.Count())
	}

	send(t, provider, "+251923456789", "Test 2", MessageTypeOTP)
	if provider.Count() != 2 {
		t.Errorf("Count() = %d, want 2", provider.Count())
	}
}

func TestMockProvider_Name(t *testing.T) {
	logger := &mockLogger{}
	provider := NewMockProvider(logger)

	if provider.Name() != "mock" {
		t.Errorf("Name() = %q, want %q", provider.Name(), "mock")
	}
}

func TestMockProvider_MessageID(t *testing.T) {
	logger := &mockLogger{}
	provider := NewMockProvider(logger)

	id, _ := provider.Send(context.Background(), "+251912345678", "Test", MessageTypeNotification)

	messages := provider.GetMessages()
	if len(messages) != 1 {
		t.Fatal("expected one message")
	}

	if messages[0].ID == "" || messages[0].ID != id {
		t.Errorf("message.ID = %q, Send returned %q", messages[0].ID, id)
	}
}

func TestMockProvider_MessageCreatedAt(t *testing.T) {
	logger := &mockLogger{}
	provider := NewMockProvider(logger)

	send(t, provider, "+251912345678", "Test", MessageTypeNotification)

	messages := provider.GetMessages()
	if len(messages) != 1 {
		t.Fatal("expected one message")
	}

	if messages[0].CreatedAt.IsZero() {
		t.Error("message.CreatedAt should be set")
	}
}

func TestMockProvider_KeepsTheMostRecent(t *testing.T) {
	provider := NewMockProvider(&mockLogger{})
	for i := range mockKeep + 5 {
		if _, err := provider.Send(context.Background(), "0911234567", fmt.Sprintf("message %d", i), MessageTypeNotification); err != nil {
			t.Fatal(err)
		}
	}
	messages := provider.GetMessages()
	if len(messages) != mockKeep {
		t.Fatalf("kept %d messages, want %d", len(messages), mockKeep)
	}
	if want := fmt.Sprintf("message %d", mockKeep+4); messages[len(messages)-1].Message != want {
		t.Errorf("newest message %q, want %q", messages[len(messages)-1].Message, want)
	}
	if want := "message 5"; messages[0].Message != want {
		t.Errorf("oldest message %q, want %q", messages[0].Message, want)
	}
}

// send sends a text a test relies on, and fails the test if it can't.
func send(t *testing.T, p *MockProvider, to, body string, kind MessageType) {
	t.Helper()
	if _, err := p.Send(context.Background(), to, body, kind); err != nil {
		t.Fatalf("sending to %s: %v", to, err)
	}
}
