package entity

import "errors"

var (
	ErrUnknownAction  = errors.New("unknown action")
	ErrEmptyStepTypes = errors.New("stepTypes is empty")

	ErrMappingRequired = errors.New("required field in mapping is not filled")

	ErrNoRecords = errors.New("got no records from database")

	ErrLimitExceeded = errors.New("limit of active tasks exceeded")
)
