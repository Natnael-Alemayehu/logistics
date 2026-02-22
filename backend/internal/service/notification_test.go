package service

import (
	"context"
	"errors"
	"testing"

	"github.com/natnael-alemayehu/logistics/pkg/sms"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSMSProvider struct {
	sendErr      error
	sendBatchErr error
	lastPhone    string
	lastMessage  string
}

func (m *mockSMSProvider) Send(ctx context.Context, to, message string, msgType sms.MessageType) error {
	m.lastPhone = to
	m.lastMessage = message
	return m.sendErr
}

func (m *mockSMSProvider) SendBatch(ctx context.Context, recipients []string, message string, msgType sms.MessageType) error {
	return m.sendBatchErr
}

func (m *mockSMSProvider) Name() string {
	return "mock"
}

type mockServiceLogger struct {
	infos  []string
	errors []string
	warns  []string
}

func (m *mockServiceLogger) Info(msg string, fields ...interface{}) {
	m.infos = append(m.infos, msg)
}

func (m *mockServiceLogger) Error(msg string, fields ...interface{}) {
	m.errors = append(m.errors, msg)
}

func (m *mockServiceLogger) Warn(msg string, fields ...interface{}) {
	m.warns = append(m.warns, msg)
}

func TestNotificationService_SendShipmentCreatedNotification(t *testing.T) {
	t.Run("sends notification successfully", func(t *testing.T) {
		sms := &mockSMSProvider{}
		logger := &mockServiceLogger{}
		svc := NewNotificationService(sms, "https://track.example.com", logger)

		err := svc.SendShipmentCreatedNotification(context.Background(), "0912345678", "TRK123")
		require.NoError(t, err)
		assert.Equal(t, "0912345678", sms.lastPhone)
		assert.Contains(t, sms.lastMessage, "TRK123")
		assert.Len(t, logger.infos, 1)
	})

	t.Run("handles SMS send error", func(t *testing.T) {
		sms := &mockSMSProvider{sendErr: errors.New("SMS failed")}
		logger := &mockServiceLogger{}
		svc := NewNotificationService(sms, "https://track.example.com", logger)

		err := svc.SendShipmentCreatedNotification(context.Background(), "0912345678", "TRK123")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "SMS failed")
		assert.Len(t, logger.errors, 1)
	})
}

func TestNotificationService_SendShipmentDispatchedNotification(t *testing.T) {
	t.Run("sends notification successfully", func(t *testing.T) {
		sms := &mockSMSProvider{}
		logger := &mockServiceLogger{}
		svc := NewNotificationService(sms, "https://track.example.com", logger)

		err := svc.SendShipmentDispatchedNotification(context.Background(), "0912345678", "TRK123")
		require.NoError(t, err)
		assert.Contains(t, sms.lastMessage, "TRK123")
		assert.Len(t, logger.infos, 1)
	})

	t.Run("handles SMS send error", func(t *testing.T) {
		sms := &mockSMSProvider{sendErr: errors.New("network error")}
		logger := &mockServiceLogger{}
		svc := NewNotificationService(sms, "https://track.example.com", logger)

		err := svc.SendShipmentDispatchedNotification(context.Background(), "0912345678", "TRK123")
		require.Error(t, err)
		assert.Len(t, logger.errors, 1)
	})
}

func TestNotificationService_SendShipmentDeliveredNotification(t *testing.T) {
	t.Run("sends notification successfully", func(t *testing.T) {
		sms := &mockSMSProvider{}
		logger := &mockServiceLogger{}
		svc := NewNotificationService(sms, "https://track.example.com", logger)

		err := svc.SendShipmentDeliveredNotification(context.Background(), "0912345678", "TRK123", "John Doe")
		require.NoError(t, err)
		assert.Contains(t, sms.lastMessage, "TRK123")
		assert.Contains(t, sms.lastMessage, "John Doe")
		assert.Len(t, logger.infos, 1)
	})

	t.Run("handles SMS send error", func(t *testing.T) {
		sms := &mockSMSProvider{sendErr: errors.New("rate limited")}
		logger := &mockServiceLogger{}
		svc := NewNotificationService(sms, "https://track.example.com", logger)

		err := svc.SendShipmentDeliveredNotification(context.Background(), "0912345678", "TRK123", "John Doe")
		require.Error(t, err)
	})
}

func TestNotificationService_SendDriverAssignmentNotification(t *testing.T) {
	t.Run("sends notification successfully", func(t *testing.T) {
		sms := &mockSMSProvider{}
		logger := &mockServiceLogger{}
		svc := NewNotificationService(sms, "https://track.example.com", logger)

		err := svc.SendDriverAssignmentNotification(context.Background(), "0912345678", "TRK123")
		require.NoError(t, err)
		assert.Equal(t, "0912345678", sms.lastPhone)
		assert.Contains(t, sms.lastMessage, "TRK123")
		assert.Len(t, logger.infos, 1)
	})

	t.Run("handles SMS send error", func(t *testing.T) {
		sms := &mockSMSProvider{sendErr: errors.New("provider unavailable")}
		logger := &mockServiceLogger{}
		svc := NewNotificationService(sms, "https://track.example.com", logger)

		err := svc.SendDriverAssignmentNotification(context.Background(), "0912345678", "TRK123")
		require.Error(t, err)
		assert.Len(t, logger.errors, 1)
	})
}

func TestNotificationService_SendDeliveryIssueNotification(t *testing.T) {
	t.Run("sends notification successfully", func(t *testing.T) {
		sms := &mockSMSProvider{}
		logger := &mockServiceLogger{}
		svc := NewNotificationService(sms, "https://track.example.com", logger)

		err := svc.SendDeliveryIssueNotification(context.Background(), "0912345678", "TRK123", "Customer not available")
		require.NoError(t, err)
		assert.Contains(t, sms.lastMessage, "TRK123")
		assert.Contains(t, sms.lastMessage, "Customer not available")
		assert.Len(t, logger.infos, 1)
	})

	t.Run("handles SMS send error", func(t *testing.T) {
		sms := &mockSMSProvider{sendErr: errors.New("failed")}
		logger := &mockServiceLogger{}
		svc := NewNotificationService(sms, "https://track.example.com", logger)

		err := svc.SendDeliveryIssueNotification(context.Background(), "0912345678", "TRK123", "Issue")
		require.Error(t, err)
	})
}

func TestNotificationService_SendBatchNotifications(t *testing.T) {
	t.Run("sends batch notifications successfully", func(t *testing.T) {
		sms := &mockSMSProvider{}
		logger := &mockServiceLogger{}
		svc := NewNotificationService(sms, "https://track.example.com", logger)

		phones := []string{"0912345678", "0712345678"}
		err := svc.SendBatchNotifications(context.Background(), phones, "TRK123", "dispatched")
		require.NoError(t, err)
		assert.Len(t, logger.infos, 1)
	})

	t.Run("returns error for unknown notification type", func(t *testing.T) {
		sms := &mockSMSProvider{}
		logger := &mockServiceLogger{}
		svc := NewNotificationService(sms, "https://track.example.com", logger)

		err := svc.SendBatchNotifications(context.Background(), []string{"0912345678"}, "TRK123", "unknown")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown notification type")
	})

	t.Run("handles batch send error", func(t *testing.T) {
		sms := &mockSMSProvider{sendBatchErr: errors.New("batch failed")}
		logger := &mockServiceLogger{}
		svc := NewNotificationService(sms, "https://track.example.com", logger)

		err := svc.SendBatchNotifications(context.Background(), []string{"0912345678"}, "TRK123", "dispatched")
		require.Error(t, err)
		assert.Len(t, logger.errors, 1)
	})
}

func TestNewNotificationService(t *testing.T) {
	sms := &mockSMSProvider{}
	logger := &mockServiceLogger{}
	trackingURL := "https://track.example.com"

	svc := NewNotificationService(sms, trackingURL, logger)

	require.NotNil(t, svc)
	assert.Equal(t, sms, svc.smsProvider)
	assert.Equal(t, trackingURL, svc.trackingURLBase)
	assert.Equal(t, logger, svc.logger)
}

func TestNotificationService_ContextCancellation(t *testing.T) {
	sms := &mockSMSProvider{}
	logger := &mockServiceLogger{}
	svc := NewNotificationService(sms, "https://track.example.com", logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := svc.SendShipmentCreatedNotification(ctx, "0912345678", "TRK123")
	_ = err
}

func BenchmarkNotificationService_SendShipmentCreatedNotification(b *testing.B) {
	sms := &mockSMSProvider{}
	logger := &mockServiceLogger{}
	svc := NewNotificationService(sms, "https://track.example.com", logger)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = svc.SendShipmentCreatedNotification(ctx, "0912345678", "TRK123")
	}
}

func BenchmarkNotificationService_SendBatchNotifications(b *testing.B) {
	sms := &mockSMSProvider{}
	logger := &mockServiceLogger{}
	svc := NewNotificationService(sms, "https://track.example.com", logger)
	ctx := context.Background()
	phones := []string{"0912345678", "0712345678", "0911111111"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = svc.SendBatchNotifications(ctx, phones, "TRK123", "dispatched")
	}
}
