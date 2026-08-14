package sms

import (
	"fmt"
	"strings"
)

type TemplateData struct {
	TrackingNumber string
	URL            string
	RecipientName  string
	ShipmentID     string
	Issue          string
}

const (
	TemplateShipmentCreated    = "shipment_created"
	TemplateShipmentDispatched = "shipment_dispatched"
	TemplateShipmentDelivered  = "shipment_delivered"
	TemplateDriverAssigned     = "driver_assigned"
	TemplateDeliveryIssue      = "delivery_issue"
)

var templates = map[string]string{
	TemplateShipmentCreated:    "Your shipment {tracking} has been created. Track at: {url}",
	TemplateShipmentDispatched: "Your shipment {tracking} is on the way.",
	TemplateShipmentDelivered:  "Your shipment {tracking} was delivered to {recipient}.",
	TemplateDriverAssigned:     "New shipment assigned: {tracking}. Open app for details.",
	TemplateDeliveryIssue:      "Delivery issue for shipment {tracking}: {issue}. Please contact support.",
}

func RenderTemplate(templateName string, data TemplateData) (string, error) {
	template, exists := templates[templateName]
	if !exists {
		return "", fmt.Errorf("unknown template: %s", templateName)
	}

	result := template
	result = strings.ReplaceAll(result, "{tracking}", data.TrackingNumber)
	result = strings.ReplaceAll(result, "{url}", data.URL)
	result = strings.ReplaceAll(result, "{recipient}", data.RecipientName)
	result = strings.ReplaceAll(result, "{shipment_id}", data.ShipmentID)
	result = strings.ReplaceAll(result, "{issue}", data.Issue)

	return result, nil
}

func ShipmentCreatedMessage(trackingNumber, trackingURL string) (string, error) {
	return RenderTemplate(TemplateShipmentCreated, TemplateData{
		TrackingNumber: trackingNumber,
		URL:            trackingURL,
	})
}

func ShipmentDispatchedMessage(trackingNumber string) (string, error) {
	return RenderTemplate(TemplateShipmentDispatched, TemplateData{
		TrackingNumber: trackingNumber,
	})
}

func ShipmentDeliveredMessage(trackingNumber, recipientName string) (string, error) {
	return RenderTemplate(TemplateShipmentDelivered, TemplateData{
		TrackingNumber: trackingNumber,
		RecipientName:  recipientName,
	})
}

func DriverAssignedMessage(trackingNumber string) (string, error) {
	return RenderTemplate(TemplateDriverAssigned, TemplateData{
		TrackingNumber: trackingNumber,
	})
}

func DeliveryIssueMessage(trackingNumber, issue string) (string, error) {
	return RenderTemplate(TemplateDeliveryIssue, TemplateData{
		TrackingNumber: trackingNumber,
		Issue:          issue,
	})
}

func GetTemplates() map[string]string {
	result := make(map[string]string)
	for k, v := range templates {
		result[k] = v
	}
	return result
}
