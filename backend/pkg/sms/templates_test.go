package sms

import (
	"strings"
	"testing"
)

func TestRenderTemplate_ShipmentCreated(t *testing.T) {
	data := TemplateData{
		TrackingNumber: "TRK123",
		URL:            "https://track.example.com/TRK123",
	}

	result, err := RenderTemplate(TemplateShipmentCreated, data)
	if err != nil {
		t.Fatalf("RenderTemplate() error = %v", err)
	}

	if !strings.Contains(result, "TRK123") {
		t.Error("result should contain tracking number")
	}
	if !strings.Contains(result, "https://track.example.com/TRK123") {
		t.Error("result should contain URL")
	}
	expected := "Your shipment TRK123 has been created. Track at: https://track.example.com/TRK123"
	if result != expected {
		t.Errorf("RenderTemplate() = %q, want %q", result, expected)
	}
}

func TestRenderTemplate_ShipmentDispatched(t *testing.T) {
	data := TemplateData{
		TrackingNumber: "TRK456",
	}

	result, err := RenderTemplate(TemplateShipmentDispatched, data)
	if err != nil {
		t.Fatalf("RenderTemplate() error = %v", err)
	}

	expected := "Your shipment TRK456 is on the way."
	if result != expected {
		t.Errorf("RenderTemplate() = %q, want %q", result, expected)
	}
}

func TestRenderTemplate_ShipmentDelivered(t *testing.T) {
	data := TemplateData{
		TrackingNumber: "TRK789",
		RecipientName:  "John Doe",
	}

	result, err := RenderTemplate(TemplateShipmentDelivered, data)
	if err != nil {
		t.Fatalf("RenderTemplate() error = %v", err)
	}

	expected := "Your shipment TRK789 was delivered to John Doe."
	if result != expected {
		t.Errorf("RenderTemplate() = %q, want %q", result, expected)
	}
}

func TestRenderTemplate_DriverAssigned(t *testing.T) {
	data := TemplateData{
		TrackingNumber: "TRK101",
	}

	result, err := RenderTemplate(TemplateDriverAssigned, data)
	if err != nil {
		t.Fatalf("RenderTemplate() error = %v", err)
	}

	expected := "New shipment assigned: TRK101. Open app for details."
	if result != expected {
		t.Errorf("RenderTemplate() = %q, want %q", result, expected)
	}
}

func TestRenderTemplate_DeliveryIssue(t *testing.T) {
	data := TemplateData{
		TrackingNumber: "TRK202",
		Issue:          "No one home",
	}

	result, err := RenderTemplate(TemplateDeliveryIssue, data)
	if err != nil {
		t.Fatalf("RenderTemplate() error = %v", err)
	}

	expected := "Delivery issue for shipment TRK202: No one home. Please contact support."
	if result != expected {
		t.Errorf("RenderTemplate() = %q, want %q", result, expected)
	}
}

func TestRenderTemplate_UnknownTemplate(t *testing.T) {
	data := TemplateData{
		TrackingNumber: "TRK999",
	}

	_, err := RenderTemplate("unknown_template", data)
	if err == nil {
		t.Error("RenderTemplate() expected error for unknown template, got nil")
	}
}

func TestShipmentCreatedMessage(t *testing.T) {
	result, err := ShipmentCreatedMessage("TRK123", "https://track.example.com/TRK123")
	if err != nil {
		t.Fatalf("ShipmentCreatedMessage() error = %v", err)
	}

	if !strings.Contains(result, "TRK123") {
		t.Error("result should contain tracking number")
	}
	if !strings.Contains(result, "https://track.example.com/TRK123") {
		t.Error("result should contain URL")
	}
}

func TestShipmentDispatchedMessage(t *testing.T) {
	result, err := ShipmentDispatchedMessage("TRK456")
	if err != nil {
		t.Fatalf("ShipmentDispatchedMessage() error = %v", err)
	}

	if !strings.Contains(result, "TRK456") {
		t.Error("result should contain tracking number")
	}
	if !strings.Contains(result, "on the way") {
		t.Error("result should contain 'on the way'")
	}
}

func TestShipmentDeliveredMessage(t *testing.T) {
	result, err := ShipmentDeliveredMessage("TRK789", "John Doe")
	if err != nil {
		t.Fatalf("ShipmentDeliveredMessage() error = %v", err)
	}

	if !strings.Contains(result, "TRK789") {
		t.Error("result should contain tracking number")
	}
	if !strings.Contains(result, "John Doe") {
		t.Error("result should contain recipient name")
	}
}

func TestDriverAssignedMessage(t *testing.T) {
	result, err := DriverAssignedMessage("TRK101")
	if err != nil {
		t.Fatalf("DriverAssignedMessage() error = %v", err)
	}

	if !strings.Contains(result, "TRK101") {
		t.Error("result should contain tracking number")
	}
	if !strings.Contains(result, "New shipment assigned") {
		t.Error("result should contain 'New shipment assigned'")
	}
}

func TestDeliveryIssueMessage(t *testing.T) {
	result, err := DeliveryIssueMessage("TRK202", "Wrong address")
	if err != nil {
		t.Fatalf("DeliveryIssueMessage() error = %v", err)
	}

	if !strings.Contains(result, "TRK202") {
		t.Error("result should contain tracking number")
	}
	if !strings.Contains(result, "Wrong address") {
		t.Error("result should contain issue")
	}
}

func TestGetTemplates(t *testing.T) {
	templates := GetTemplates()

	expectedTemplates := []string{
		TemplateShipmentCreated,
		TemplateShipmentDispatched,
		TemplateShipmentDelivered,
		TemplateDriverAssigned,
		TemplateDeliveryIssue,
	}

	for _, tmpl := range expectedTemplates {
		if _, exists := templates[tmpl]; !exists {
			t.Errorf("GetTemplates() missing template %q", tmpl)
		}
	}

	if len(templates) != len(expectedTemplates) {
		t.Errorf("GetTemplates() returned %d templates, want %d", len(templates), len(expectedTemplates))
	}
}

func TestTemplatePlaceholdersReplaced(t *testing.T) {
	tests := []struct {
		name             string
		templateName     string
		data             TemplateData
		shouldContain    []string
		shouldNotContain []string
	}{
		{
			name:         "all placeholders in shipment created",
			templateName: TemplateShipmentCreated,
			data: TemplateData{
				TrackingNumber: "TRACK123",
				URL:            "https://example.com",
			},
			shouldContain:    []string{"TRACK123", "https://example.com"},
			shouldNotContain: []string{"{tracking}", "{url}"},
		},
		{
			name:         "recipient placeholder in delivered",
			templateName: TemplateShipmentDelivered,
			data: TemplateData{
				TrackingNumber: "TRACK456",
				RecipientName:  "Alice",
			},
			shouldContain:    []string{"TRACK456", "Alice"},
			shouldNotContain: []string{"{tracking}", "{recipient}"},
		},
		{
			name:         "issue placeholder in delivery issue",
			templateName: TemplateDeliveryIssue,
			data: TemplateData{
				TrackingNumber: "TRACK789",
				Issue:          "Customer unavailable",
			},
			shouldContain:    []string{"TRACK789", "Customer unavailable"},
			shouldNotContain: []string{"{tracking}", "{issue}"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := RenderTemplate(tt.templateName, tt.data)
			if err != nil {
				t.Fatalf("RenderTemplate() error = %v", err)
			}

			for _, s := range tt.shouldContain {
				if !strings.Contains(result, s) {
					t.Errorf("result should contain %q, got %q", s, result)
				}
			}

			for _, s := range tt.shouldNotContain {
				if strings.Contains(result, s) {
					t.Errorf("result should not contain %q, got %q", s, result)
				}
			}
		})
	}
}
