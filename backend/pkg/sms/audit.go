package sms

import (
	"context"
	"time"
)

type AuditLogger interface {
	LogSMS(ctx context.Context, entry AuditEntry) error
}

type AuditEntry struct {
	ID            string
	Phone         string
	Message       string
	MessageType   MessageType
	Status        string
	Provider      string
	Error         string
	RetryCount    int
	Duration      time.Duration
	TenantID      string
	ShipmentID    string
	CorrelationID string
	CreatedAt     time.Time
}

type InMemoryAuditLogger struct {
	entries []AuditEntry
	mutex   chan struct{}
}

func NewInMemoryAuditLogger() *InMemoryAuditLogger {
	return &InMemoryAuditLogger{
		entries: make([]AuditEntry, 0),
		mutex:   make(chan struct{}, 1),
	}
}

func (l *InMemoryAuditLogger) LogSMS(ctx context.Context, entry AuditEntry) error {
	l.mutex <- struct{}{}
	defer func() { <-l.mutex }()

	entry.CreatedAt = time.Now()
	l.entries = append(l.entries, entry)
	return nil
}

func (l *InMemoryAuditLogger) GetEntries() []AuditEntry {
	l.mutex <- struct{}{}
	defer func() { <-l.mutex }()

	result := make([]AuditEntry, len(l.entries))
	copy(result, l.entries)
	return result
}

func (l *InMemoryAuditLogger) GetEntriesByPhone(phone string) []AuditEntry {
	l.mutex <- struct{}{}
	defer func() { <-l.mutex }()

	var result []AuditEntry
	for _, entry := range l.entries {
		if entry.Phone == phone {
			result = append(result, entry)
		}
	}
	return result
}

func (l *InMemoryAuditLogger) Clear() {
	l.mutex <- struct{}{}
	defer func() { <-l.mutex }()

	l.entries = make([]AuditEntry, 0)
}
