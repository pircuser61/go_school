package errorutils

import (
	"context"
	"errors"
)

// специальная ошибка предназначенная для обработки неудачных запросов к внешним системам
//
//	err := remoteCall(ctx)
//	if err != nil {
//		return errors.Join(ErrRemoteCallFailed, err)
//	}
var ErrRemoteCallFailed error = RemoteCallError{}

type RemoteCallError struct{}

func (e RemoteCallError) Error() string {
	return "request canceled or external system is not available"
}

func IsRemoteCallError(err error) bool {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return true
	case errors.Is(err, context.Canceled):
		return true
	default:
		return errors.Is(err, ErrRemoteCallFailed)
	}
}
