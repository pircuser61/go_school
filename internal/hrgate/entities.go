package hrgate

import (
	"time"

	"golang.org/x/exp/slices"
)

type CalendarDays struct {
	Holidays    []int64 `json:"holidays"`
	PreHolidays []int64 `json:"pre_holidays"`
	WorkDay     []int64 `json:"work_day"`
	Weekend     []int64 `json:"weekend"`
}

func (cd *CalendarDays) GetDayType(dayTime time.Time) CalendarDayType { // it takes unix time, and we need it to convert to unix time at 00:00 am of day
	if cd == nil {
		return CalendarDayTypeWorkday
	}

	year, month, day := dayTime.Date()
	unixTime := time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Unix() // does not matter what timezone it is
	if slices.Contains(cd.Holidays, unixTime) {
		return CalendarDayTypeHoliday
	} else if slices.Contains(cd.PreHolidays, unixTime) {
		return CalendarDayTypePreHoliday
	} else if slices.Contains(cd.Weekend, unixTime) {
		return CalendarDayTypeWeekend
	} else {
		return CalendarDayTypeWorkday
	}
}
