package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type PipelinerErrorCode int

const (
	UnknownError PipelinerErrorCode = iota
	GetApprovedError
	GetOnApproveError
	)

var errorText  = map[PipelinerErrorCode]string{
	UnknownError: "unknown error",
	GetApprovedError: "can't get approved versions",
	GetOnApproveError: "can't get versions on approve",
}


var errorDescription  = map[PipelinerErrorCode]string{
	UnknownError: "Сохраняйте спокойствие, что-то произошло непонятное",
	GetApprovedError: "Невозможно получить список согласованных сценариев",
	GetOnApproveError: "Невозможно получить список сценариев, ожидающих согласования",
}


type httpError struct {
	StatusCode int
	Error string
	Description string
}

func (c PipelinerErrorCode) errorMessage(e error) string {
	return fmt.Sprintf("%s: %s", errorText[c], e)
}

func (c PipelinerErrorCode) sendError(w http.ResponseWriter) error {
	statusCode := http.StatusInternalServerError
	resp := httpError{
		StatusCode: statusCode,
		Error:       errorText[c],
		Description: errorDescription[c],
	}
	w.WriteHeader(statusCode)
	err := json.NewEncoder(w).Encode(resp)
	if err != nil {
		return err
	}
	return nil
}