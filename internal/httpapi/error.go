package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

type ErrorDetails interface {
	errorDetails()
}

type ValidationDetails struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

func (ValidationDetails) errorDetails() {}

type NotFoundDetails struct {
	Resource string `json:"resource"`
	ID       string `json:"id"`
}

func (NotFoundDetails) errorDetails() {}

type RateLimitDetails struct {
	RetryAfter int `json:"retryAfter"` // seconds
	Limit      int `json:"limit"`
	Remaining  int `json:"remaining"`
}

func (RateLimitDetails) errorDetails() {}

type AuthDetails struct {
	Reason string `json:"reason"` // "expired", "missing", "invalid"
}

func (AuthDetails) errorDetails() {}

type ConflictDetails struct {
	Resource string `json:"resource"`
	Field    string `json:"field"`
	Value    string `json:"value"`
}

func (ConflictDetails) errorDetails() {}

type MultiValidationDetails struct {
	Errors []ValidationDetails `json:"errors"`
}

func (MultiValidationDetails) errorDetails() {}

type errorBody struct {
	Code      string       `json:"code"`
	Message   string       `json:"message"`
	RequestId string       `json:"requestId"`
	Details   ErrorDetails `json:"details,omitempty"`
}

func WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string, details ErrorDetails) {
	WriteJSON(w, status, envelope[struct{}]{
		Error: &errorBody{
			Code:      code,
			Message:   message,
			Details:   details,
			RequestId: middleware.GetReqID(r.Context()),
		},
	})
}
