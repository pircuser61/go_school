package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type PipelinerErrorCode int

const (
	UnknownError PipelinerErrorCode = iota
	UUIDParsingError
	RequestReadError
	PipelineParseError
	PipelineReadError
	PipelineWriteError
	GetAllApprovedError
	GetAllOnApproveError
	GetAllDraftsError
	GetPipelineError
	GetVersionError
	PipelineIsDraft
	ApproveError
	PipelineDeleteError
	PipelineCreateError
	)

var errorText  = map[PipelinerErrorCode]string{
	UnknownError:         "unknown error",
	GetAllApprovedError:  "can't get approved versions",
	GetAllOnApproveError: "can't get versions on approve",
	GetAllDraftsError: "can't get draft versions",
	UUIDParsingError: "can't find uuid",
	GetPipelineError: "can't get pipeline",
	GetVersionError: "can't get pipeline version",
	RequestReadError: "can't read request",
	PipelineReadError: "can't read pipeline data",
	PipelineWriteError: "can't write pipeline data",
	PipelineParseError: "can't pars pipeline data",
	PipelineIsDraft: "pipeline is not a draft",
	ApproveError: "can't approve pipeline",
	PipelineDeleteError: "can't delete pipeline data",
	PipelineCreateError: "can't create pipeline",
}


var errorDescription  = map[PipelinerErrorCode]string{
	UnknownError:         "Сохраняйте спокойствие, что-то произошло непонятное",
	GetAllApprovedError:  "Невозможно получить список согласованных сценариев",
	GetAllOnApproveError: "Невозможно получить список сценариев, ожидающих согласования",
	GetAllDraftsError: "Невозможно получить список редактируемых сценариев",
	UUIDParsingError: "Не удалось прочитать идентификатор",
	GetPipelineError:"Не удалось получить информацию о сценарии",
	GetVersionError: "Не удалось получить информацию о сценарии",
	RequestReadError: "Не удалось прочитать запрос",
	PipelineReadError: "Не удалось прочитать информацию о сценарии",
	PipelineIsDraft: "Редактирование согласованного сценария запрещено, необходимо создать новую версию",
	PipelineWriteError: "Не удалось записать информацию о сценарии",
	PipelineParseError: "Не удалось разобрать информацию о сценарии",
	ApproveError: "Не удалось согласовать сценарий",
	PipelineDeleteError: "Не удалось удалить информацию о сценарии",
	PipelineCreateError: "Не удалось создать информацию о сценарии",
}


type httpError struct {
	StatusCode int `json:"status_code""`
	Error string `json:"error"`
	Description string `json:"description"`
}

func (c PipelinerErrorCode) errorMessage(e error) string {
	return fmt.Sprintf("%s: %s", errorText[c], e.Error())
}

func (c PipelinerErrorCode) error() string  {
	s, ok := errorText[c]
	if ok {
		return s
	}
	return errorText[UnknownError]
}

func (c PipelinerErrorCode)  description() string {
	s, ok := errorDescription[c]
	if ok {
		return s
	}
	return errorDescription[UnknownError]
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