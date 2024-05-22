package errorutils

import (
	"context"
	"errors"
)

var ErrExternalSystemIsNotAvailable error = ExternalSystemError{}

type ExternalSystemError struct{}

func (e ExternalSystemError) Error() string {
	return "request canceled or external system is not available"
}

func IsExternalSystemError(err error) bool {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return true
	case errors.Is(err, context.Canceled):
		return true
	default:
		return errors.Is(err, ErrExternalSystemIsNotAvailable)
	}
}
