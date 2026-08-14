package response

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5/middleware"
)

func TestJSON(t *testing.T) {
	t.Run("writes correct status code", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		JSON(w, r, http.StatusCreated, map[string]string{"foo": "bar"})

		if w.Code != http.StatusCreated {
			t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
		}
	})

	t.Run("sets Content-Type header", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		JSON(w, r, http.StatusOK, map[string]string{"foo": "bar"})

		contentType := w.Header().Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("expected Content-Type %q, got %q", "application/json", contentType)
		}
	})

	t.Run("encodes data correctly", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		data := map[string]interface{}{"name": "test", "value": 42}

		JSON(w, r, http.StatusOK, data)

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		dataMap, ok := resp.Data.(map[string]interface{})
		if !ok {
			t.Fatalf("expected data to be map, got %T", resp.Data)
		}
		if dataMap["name"] != "test" {
			t.Errorf("expected name %q, got %v", "test", dataMap["name"])
		}
		if dataMap["value"].(float64) != 42 {
			t.Errorf("expected value %d, got %v", 42, dataMap["value"])
		}
	})

	t.Run("includes request ID in meta when present", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		ctx := context.WithValue(r.Context(), middleware.RequestIDKey, "test-request-123")
		r = r.WithContext(ctx)

		JSON(w, r, http.StatusOK, map[string]string{"foo": "bar"})

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if resp.Meta == nil {
			t.Fatal("expected meta to be non-nil")
		}
		if resp.Meta.RequestID != "test-request-123" {
			t.Errorf("expected request ID %q, got %q", "test-request-123", resp.Meta.RequestID)
		}
	})

	t.Run("omits meta when request ID is not present", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		JSON(w, r, http.StatusOK, map[string]string{"foo": "bar"})

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if resp.Meta != nil {
			t.Errorf("expected meta to be nil, got %+v", resp.Meta)
		}
	})
}

func TestErrorJSON(t *testing.T) {
	t.Run("writes correct status code", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		ErrorJSON(w, r, http.StatusBadRequest, CodeValidationError, "Invalid input")

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
		}
	})

	t.Run("formats error response correctly", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		ErrorJSON(w, r, http.StatusNotFound, CodeNotFound, "Item not found")

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if resp.Error == nil {
			t.Fatal("expected error to be non-nil")
		}
		if resp.Error.Code != CodeNotFound {
			t.Errorf("expected code %q, got %q", CodeNotFound, resp.Error.Code)
		}
		if resp.Error.Message != "Item not found" {
			t.Errorf("expected message %q, got %q", "Item not found", resp.Error.Message)
		}
		if resp.Error.Details != nil {
			t.Errorf("expected details to be nil, got %v", resp.Error.Details)
		}
	})

	t.Run("includes optional details parameter", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		details := map[string]string{"field": "email", "error": "invalid format"}

		ErrorJSON(w, r, http.StatusBadRequest, CodeValidationError, "Validation failed", details)

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if resp.Error.Details == nil {
			t.Fatal("expected details to be non-nil")
		}
		detailsMap, ok := resp.Error.Details.(map[string]interface{})
		if !ok {
			t.Fatalf("expected details to be map, got %T", resp.Error.Details)
		}
		if detailsMap["field"] != "email" {
			t.Errorf("expected field %q, got %v", "email", detailsMap["field"])
		}
	})

	t.Run("includes request ID in meta", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		ctx := context.WithValue(r.Context(), middleware.RequestIDKey, "error-req-456")
		r = r.WithContext(ctx)

		ErrorJSON(w, r, http.StatusInternalServerError, CodeInternalError, "Something went wrong")

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if resp.Meta == nil {
			t.Fatal("expected meta to be non-nil")
		}
		if resp.Meta.RequestID != "error-req-456" {
			t.Errorf("expected request ID %q, got %q", "error-req-456", resp.Meta.RequestID)
		}
	})

	t.Run("sets Content-Type header", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		ErrorJSON(w, r, http.StatusBadRequest, CodeValidationError, "Invalid input")

		contentType := w.Header().Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("expected Content-Type %q, got %q", "application/json", contentType)
		}
	})
}

func TestPaginatedJSON(t *testing.T) {
	t.Run("includes pagination metadata", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		data := []map[string]string{{"id": "1"}, {"id": "2"}}

		PaginatedJSON(w, r, http.StatusOK, data, 2, 10, 25)

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if resp.Meta == nil {
			t.Fatal("expected meta to be non-nil")
		}
		if resp.Meta.Page != 2 {
			t.Errorf("expected page %d, got %d", 2, resp.Meta.Page)
		}
		if resp.Meta.PerPage != 10 {
			t.Errorf("expected per_page %d, got %d", 10, resp.Meta.PerPage)
		}
		if resp.Meta.Total != 25 {
			t.Errorf("expected total %d, got %d", 25, resp.Meta.Total)
		}
	})

	t.Run("encodes data correctly", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		data := []map[string]string{{"id": "1", "name": "first"}, {"id": "2", "name": "second"}}

		PaginatedJSON(w, r, http.StatusOK, data, 1, 10, 2)

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		items, ok := resp.Data.([]interface{})
		if !ok {
			t.Fatalf("expected data to be slice, got %T", resp.Data)
		}
		if len(items) != 2 {
			t.Errorf("expected %d items, got %d", 2, len(items))
		}
	})

	t.Run("writes correct status code", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		PaginatedJSON(w, r, http.StatusOK, nil, 1, 10, 0)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}
	})

	t.Run("sets Content-Type header", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		PaginatedJSON(w, r, http.StatusOK, nil, 1, 10, 0)

		contentType := w.Header().Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("expected Content-Type %q, got %q", "application/json", contentType)
		}
	})

	t.Run("includes request ID in meta", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		ctx := context.WithValue(r.Context(), middleware.RequestIDKey, "paginated-req-789")
		r = r.WithContext(ctx)

		PaginatedJSON(w, r, http.StatusOK, nil, 1, 10, 0)

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if resp.Meta.RequestID != "paginated-req-789" {
			t.Errorf("expected request ID %q, got %q", "paginated-req-789", resp.Meta.RequestID)
		}
	})
}

func TestNotFound(t *testing.T) {
	t.Run("returns correct status code", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		NotFound(w, r, "")

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
		}
	})

	t.Run("uses default message when empty", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		NotFound(w, r, "")

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if resp.Error.Message != "Resource not found" {
			t.Errorf("expected message %q, got %q", "Resource not found", resp.Error.Message)
		}
		if resp.Error.Code != CodeNotFound {
			t.Errorf("expected code %q, got %q", CodeNotFound, resp.Error.Code)
		}
	})

	t.Run("uses custom message when provided", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		NotFound(w, r, "User with ID 123 not found")

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if resp.Error.Message != "User with ID 123 not found" {
			t.Errorf("expected message %q, got %q", "User with ID 123 not found", resp.Error.Message)
		}
	})
}

func TestUnauthorized(t *testing.T) {
	t.Run("returns correct status code", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		Unauthorized(w, r, "")

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
		}
	})

	t.Run("uses default message when empty", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		Unauthorized(w, r, "")

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if resp.Error.Message != "Unauthorized" {
			t.Errorf("expected message %q, got %q", "Unauthorized", resp.Error.Message)
		}
		if resp.Error.Code != CodeUnauthorized {
			t.Errorf("expected code %q, got %q", CodeUnauthorized, resp.Error.Code)
		}
	})

	t.Run("uses custom message when provided", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		Unauthorized(w, r, "Invalid API key")

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if resp.Error.Message != "Invalid API key" {
			t.Errorf("expected message %q, got %q", "Invalid API key", resp.Error.Message)
		}
	})
}

func TestForbidden(t *testing.T) {
	t.Run("returns correct status code", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		Forbidden(w, r, "")

		if w.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d", http.StatusForbidden, w.Code)
		}
	})

	t.Run("uses default message when empty", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		Forbidden(w, r, "")

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if resp.Error.Message != "Forbidden" {
			t.Errorf("expected message %q, got %q", "Forbidden", resp.Error.Message)
		}
		if resp.Error.Code != CodeForbidden {
			t.Errorf("expected code %q, got %q", CodeForbidden, resp.Error.Code)
		}
	})

	t.Run("uses custom message when provided", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		Forbidden(w, r, "You do not have admin privileges")

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if resp.Error.Message != "You do not have admin privileges" {
			t.Errorf("expected message %q, got %q", "You do not have admin privileges", resp.Error.Message)
		}
	})
}

func TestRateLimited(t *testing.T) {
	t.Run("returns correct status code", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		RateLimited(w, r)

		if w.Code != http.StatusTooManyRequests {
			t.Errorf("expected status %d, got %d", http.StatusTooManyRequests, w.Code)
		}
	})

	t.Run("returns correct error code and message", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		RateLimited(w, r)

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if resp.Error.Message != "Rate limit exceeded" {
			t.Errorf("expected message %q, got %q", "Rate limit exceeded", resp.Error.Message)
		}
		if resp.Error.Code != CodeRateLimited {
			t.Errorf("expected code %q, got %q", CodeRateLimited, resp.Error.Code)
		}
	})
}

func TestInternalError(t *testing.T) {
	t.Run("returns correct status code", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		InternalError(w, r, "")

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
		}
	})

	t.Run("uses default message when empty", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		InternalError(w, r, "")

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if resp.Error.Message != "Internal server error" {
			t.Errorf("expected message %q, got %q", "Internal server error", resp.Error.Message)
		}
		if resp.Error.Code != CodeInternalError {
			t.Errorf("expected code %q, got %q", CodeInternalError, resp.Error.Code)
		}
	})

	t.Run("uses custom message when provided", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		InternalError(w, r, "Database connection failed")

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if resp.Error.Message != "Database connection failed" {
			t.Errorf("expected message %q, got %q", "Database connection failed", resp.Error.Message)
		}
	})
}

func TestValidationErrors(t *testing.T) {
	t.Run("returns correct status code", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		ValidationErrors(w, r, nil)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
		}
	})

	t.Run("returns correct error code and message", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		ValidationErrors(w, r, nil)

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if resp.Error.Message != "Validation failed" {
			t.Errorf("expected message %q, got %q", "Validation failed", resp.Error.Message)
		}
		if resp.Error.Code != CodeValidationError {
			t.Errorf("expected code %q, got %q", CodeValidationError, resp.Error.Code)
		}
	})

	t.Run("includes validation details", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		errors := []map[string]string{
			{"field": "email", "error": "required"},
			{"field": "password", "error": "too short"},
		}

		ValidationErrors(w, r, errors)

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if resp.Error.Details == nil {
			t.Fatal("expected details to be non-nil")
		}
	})
}
