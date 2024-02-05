package utils

import (
	"time"
)

type TimeUnit string

const (
	Day         TimeUnit = "day"
	Hour        TimeUnit = "hour"
	Minute      TimeUnit = "minute"
	Second      TimeUnit = "second"
	Microsecond TimeUnit = "microsecond"
	Millisecond TimeUnit = "millisecond"
	Nanosecond  TimeUnit = "nanosecond"
)

func GetDateUnitNumBetweenDates(startDate, endDate time.Time, unit TimeUnit) float64 {
	duration := endDate.Sub(startDate)

	switch unit {
	case Day:
		r := duration.Hours() / 24.0

		return r
	case Hour:
		r := duration.Hours()

		return r
	case Minute:
		r := duration.Minutes()

		return r
	case Second:
		r := duration.Seconds()

		return r
	case Microsecond:
		r := float64(duration.Microseconds())

		return r
	case Millisecond:
		r := float64(duration.Milliseconds())

		return r
	case Nanosecond:
		r := float64(duration.Nanoseconds())

		return r
	default:
		return float64(0)
	}
}
