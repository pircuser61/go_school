package hrgate

import (
	"time"
)

type CalendarDays struct {
	CalendarMap map[int64]CalendarDayType
	// No weekend needs because we check it in other places of code
}

func (cd *CalendarDays) GetDayType(dayTime time.Time) (dayType CalendarDayType, foundInCalendar bool) {
	// it takes unix time, and we need it to convert to unix time at 00:00 am of day
	if cd == nil {
		return CalendarDayTypeWorkday, false
	}

	year, month, day := dayTime.Date()
	unixTime := time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Unix()
	// because calendar days returned at utc timezone

	if calendarDayType, ok := cd.CalendarMap[unixTime]; ok {
		return calendarDayType, true
	}

	return CalendarDayTypeWorkday, false
}
