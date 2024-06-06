package errorutils

import (
	"context"
	"errors"
	"io"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrRemoteCallFailed - специальная ошибка предназначенная для обработки неудачных запросов к внешним системам
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
	// grpc коды
	switch status.Code(err) {
	case codes.Unavailable, codes.DeadlineExceeded:
		return true
	}

	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return true
	case errors.Is(err, context.Canceled):
		return true
	// ErrUnexpectedEOF возвращается из базы если соединение прерывается
	case errors.Is(err, io.ErrUnexpectedEOF):
		return true
	// Для кастомного ретрая
	default:
		return errors.Is(err, ErrRemoteCallFailed)
	}
}
