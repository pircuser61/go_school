package hrgate

import (
	"net/http"
	"strconv"
)

const (
	totalHeader  = "total"
	offsetHeader = "offset"
	limitHeader  = "limit"
)

func handleHeaders(hh http.Header) (total, offset, limit int, err error) {
	currTotal := hh.Get(totalHeader)
	total, err = strconv.Atoi(currTotal)
	if err != nil {
		return 0, 0, 0, err
	}

	currOffset := hh.Get(offsetHeader)
	offset, err = strconv.Atoi(currOffset)
	if err != nil {
		return 0, 0, 0, err
	}

	currLimit := hh.Get(limitHeader)
	limit, err = strconv.Atoi(currLimit)
	if err != nil {
		return 0, 0, 0, err
	}

	return
}
