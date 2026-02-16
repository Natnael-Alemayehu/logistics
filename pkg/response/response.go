package response

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

type Response struct {
	Data  interface{} `json:"data,omitempty"`
	Error *Error      `json:"error,omitempty"`
	Meta  *Meta       `json:"meta,omitempty"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

type Meta struct {
	RequestID string `json:"request_id,omitempty"`
	Page      int    `json:"page,omitempty"`
	PerPage   int    `json:"per_page,omitempty"`
	Total     int    `json:"total,omitempty"`
}

func JSON(w http.ResponseWriter, r *http.Request, status int, data interface{}) {
	resp := Response{Data: data}
	if requestID := middleware.GetReqID(r.Context()); requestID != "" {
		resp.Meta = &Meta{RequestID: requestID}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

func ErrorJSON(w http.ResponseWriter, r *http.Request, status int, code, message string, details ...any) {
	resp := Response{
		Error: &Error{
			Code:    code,
			Message: message,
		},
	}
	if len(details) > 0 {
		resp.Error.Details = details[0]
	}
	if requestID := middleware.GetReqID(r.Context()); requestID != "" {
		resp.Meta = &Meta{RequestID: requestID}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

func PaginatedJSON(w http.ResponseWriter, r *http.Request, status int, data interface{}, page, perPage, total int) {
	resp := Response{
		Data: data,
		Meta: &Meta{
			RequestID: middleware.GetReqID(r.Context()),
			Page:      page,
			PerPage:   perPage,
			Total:     total,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}
