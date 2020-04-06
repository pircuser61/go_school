package app

import (
	"encoding/json"
	"net/http"
)

type httpResponse struct {
	StatusCode int         `json:"status_code"`
	Data       interface{} `json:"data"`
}

func sendResponse(w http.ResponseWriter, statusCode int, body interface{}) error {
	resp := httpResponse{
		StatusCode: statusCode,
		Data:       body,
	}
	w.WriteHeader(statusCode)
	err := json.NewEncoder(w).Encode(resp)
	if err != nil {
		return err
	}
	return nil
}

func sendError(w http.ResponseWriter, e error) {
	errorString := e.Error()
	_ = sendResponse(w, 503, errorString)
}
