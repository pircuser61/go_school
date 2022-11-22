package pipeline

import (
	"github.com/pkg/errors"
)

var (
	ErrCantGetNextStep = errors.New("can't get next step")
)
