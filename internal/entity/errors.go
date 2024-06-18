package entity

import "errors"

var (
	ErrUnknownAction  = errors.New("unknown action")
	ErrEmptyStepTypes = errors.New("stepTypes is empty")

	ErrNoRecords = errors.New("got no records from database")
)
