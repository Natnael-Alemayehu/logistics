package service

import (
	"context"
	"fmt"

	"github.com/natnael-alemayehu/logistics/pkg/sms"
)

type NotificationService struct {
	smsProvider     sms.SMSProvider
	trackingURLBase string
	logger          Logger
}

type Logger interface {
	Info(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
}

func NewNotificationService(smsProvider sms.SMSProvider, trackingURLBase string, logger Logger) *NotificationService {
	return &NotificationService{
		smsProvider:     smsProvider,
		trackingURLBase: trackingURLBase,
		logger:          logger,
	}
}

func (s *NotificationService) SendShipmentCreatedNotification(ctx context.Context, customerPhone, trackingNumber string) error {
	trackingURL := fmt.Sprintf("%s/track/%s", s.trackingURLBase, trackingNumber)
	message, err := sms.ShipmentCreatedMessage(trackingNumber, trackingURL)
	if err != nil {
		return fmt.Errorf("failed to create shipment created message: %w", err)
	}

	if err := s.smsProvider.Send(ctx, customerPhone, message, sms.MessageTypeNotification); err != nil {
		s.logger.Error("failed to send shipment created notification", "phone", customerPhone, "tracking", trackingNumber, "error", err)
		return err
	}

	s.logger.Info("shipment created notification sent", "phone", customerPhone, "tracking", trackingNumber)
	return nil
}

func (s *NotificationService) SendShipmentDispatchedNotification(ctx context.Context, customerPhone, trackingNumber string) error {
	message, err := sms.ShipmentDispatchedMessage(trackingNumber)
	if err != nil {
		return fmt.Errorf("failed to create shipment dispatched message: %w", err)
	}

	if err := s.smsProvider.Send(ctx, customerPhone, message, sms.MessageTypeNotification); err != nil {
		s.logger.Error("failed to send shipment dispatched notification", "phone", customerPhone, "tracking", trackingNumber, "error", err)
		return err
	}

	s.logger.Info("shipment dispatched notification sent", "phone", customerPhone, "tracking", trackingNumber)
	return nil
}

func (s *NotificationService) SendShipmentDeliveredNotification(ctx context.Context, customerPhone, trackingNumber, recipientName string) error {
	message, err := sms.ShipmentDeliveredMessage(trackingNumber, recipientName)
	if err != nil {
		return fmt.Errorf("failed to create shipment delivered message: %w", err)
	}

	if err := s.smsProvider.Send(ctx, customerPhone, message, sms.MessageTypeNotification); err != nil {
		s.logger.Error("failed to send shipment delivered notification", "phone", customerPhone, "tracking", trackingNumber, "error", err)
		return err
	}

	s.logger.Info("shipment delivered notification sent", "phone", customerPhone, "tracking", trackingNumber)
	return nil
}

func (s *NotificationService) SendDriverAssignmentNotification(ctx context.Context, driverPhone, trackingNumber string) error {
	message, err := sms.DriverAssignedMessage(trackingNumber)
	if err != nil {
		return fmt.Errorf("failed to create driver assignment message: %w", err)
	}

	if err := s.smsProvider.Send(ctx, driverPhone, message, sms.MessageTypeAlert); err != nil {
		s.logger.Error("failed to send driver assignment notification", "phone", driverPhone, "tracking", trackingNumber, "error", err)
		return err
	}

	s.logger.Info("driver assignment notification sent", "phone", driverPhone, "tracking", trackingNumber)
	return nil
}

func (s *NotificationService) SendDeliveryIssueNotification(ctx context.Context, dispatcherPhone, trackingNumber, issue string) error {
	message, err := sms.DeliveryIssueMessage(trackingNumber, issue)
	if err != nil {
		return fmt.Errorf("failed to create delivery issue message: %w", err)
	}

	if err := s.smsProvider.Send(ctx, dispatcherPhone, message, sms.MessageTypeAlert); err != nil {
		s.logger.Error("failed to send delivery issue notification", "phone", dispatcherPhone, "tracking", trackingNumber, "error", err)
		return err
	}

	s.logger.Info("delivery issue notification sent", "phone", dispatcherPhone, "tracking", trackingNumber)
	return nil
}

func (s *NotificationService) SendBatchNotifications(ctx context.Context, phones []string, trackingNumber string, notificationType string) error {
	var message string
	var err error

	switch notificationType {
	case "dispatched":
		message, err = sms.ShipmentDispatchedMessage(trackingNumber)
	default:
		return fmt.Errorf("unknown notification type: %s", notificationType)
	}

	if err != nil {
		return fmt.Errorf("failed to create message: %w", err)
	}

	if err := s.smsProvider.SendBatch(ctx, phones, message, sms.MessageTypeNotification); err != nil {
		s.logger.Error("failed to send batch notifications", "tracking", trackingNumber, "error", err)
		return err
	}

	s.logger.Info("batch notifications sent", "count", len(phones), "tracking", trackingNumber)
	return nil
}
