package sms

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/natnael-alemayehu/logistics/pkg/console"
)

// MockProvider stands in for a gateway: messages are kept, and printed to
// Console when one is set, instead of being sent.
type MockProvider struct {
	Console  *console.Console
	logger   Logger
	messages []MockMessage
	mutex    sync.RWMutex
}

type MockMessage struct {
	ID        string
	To        string
	Message   string
	Type      MessageType
	Status    string
	CreatedAt time.Time
}

func NewMockProvider(logger Logger) *MockProvider {
	return &MockProvider{
		logger:   logger,
		messages: make([]MockMessage, 0),
	}
}

func (m *MockProvider) Name() string {
	return "mock"
}

func (m *MockProvider) Send(ctx context.Context, to, message string, msgType MessageType) error {
	normalizedPhone, err := NormalizeEthiopianPhone(to)
	if err != nil {
		m.logger.Error("invalid phone number", "phone", to, "error", err)
		return err
	}

	msg := MockMessage{
		ID:        fmt.Sprintf("mock-%d", time.Now().UnixNano()),
		To:        normalizedPhone,
		Message:   message,
		Type:      msgType,
		Status:    "sent",
		CreatedAt: time.Now(),
	}

	m.mutex.Lock()
	m.messages = append(m.messages, msg)
	m.mutex.Unlock()

	if m.Console != nil {
		m.Console.Print(console.Item{Kind: "sms", To: to, Body: message, At: msg.CreatedAt})
	} else {
		m.logger.Info("[MOCK SMS] Message sent", "id", msg.ID, "to", normalizedPhone, "type", msgType, "body", message)
	}
	return nil
}

func (m *MockProvider) SendBatch(ctx context.Context, recipients []string, message string, msgType MessageType) error {
	for _, recipient := range recipients {
		if err := m.Send(ctx, recipient, message, msgType); err != nil {
			m.logger.Error("[MOCK SMS] Failed to send to recipient", "recipient", recipient, "error", err)
		}
	}
	return nil
}

func (m *MockProvider) GetMessages() []MockMessage {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := make([]MockMessage, len(m.messages))
	copy(result, m.messages)
	return result
}

func (m *MockProvider) GetMessagesByPhone(phone string) []MockMessage {
	normalized, err := NormalizeEthiopianPhone(phone)
	if err != nil {
		return nil
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var result []MockMessage
	for _, msg := range m.messages {
		if msg.To == normalized {
			result = append(result, msg)
		}
	}
	return result
}

func (m *MockProvider) GetLastMessage() *MockMessage {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if len(m.messages) == 0 {
		return nil
	}
	return &m.messages[len(m.messages)-1]
}

func (m *MockProvider) Clear() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.messages = make([]MockMessage, 0)
}

func (m *MockProvider) Count() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return len(m.messages)
}
