package handlers

import (
	"encoding/json"
	"net/http"
)

type httpResponse struct {
	StatusCode int         `json:"status_code"`
	Data       interface{} `json:"data,omitempty"`
}

//nolint:unparam //todo may be used later
func sendResponse(w http.ResponseWriter, statusCode int, body interface{}) error {
	resp := httpResponse{
		StatusCode: statusCode,
		Data:       body,
	}

	w.WriteHeader(statusCode)

	err := json.NewEncoder(w).Encode(resp)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		return err
	}

	return nil
}
