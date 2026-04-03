package handlers

import "encoding/json"

type apiErrorEnvelope struct {
	Error *apiErrorBody `json:"error"`
}

type apiErrorBody struct {
	Code      string          `json:"code"`
	Message   string          `json:"message"`
	RequestID string          `json:"requestId"`
	Details   json.RawMessage `json:"details,omitempty"`
}
