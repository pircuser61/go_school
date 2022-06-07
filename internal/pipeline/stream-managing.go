package pipeline

import (
	"math"

	"github.com/pkg/errors"
)

var (
	ErrCantGetNextStep    = errors.New("can't get next step")
	errCantCastIndexToInt = errors.New("can't cast index to int")
)

func indexToInt(i interface{}) (int, bool) {
	switch i.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		index, ok := i.(int)

		return index, ok
	case float32, float64:
		floatIndex, ok := i.(float64)
		index := int(math.Round(floatIndex))

		return index, ok
	default:
		return 0, false
	}
}
