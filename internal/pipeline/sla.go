package pipeline

import "time"

func beforeWorktime(t time.Time) bool {
	return t.Hour() < 6
}

func afterWorktime(t time.Time) bool {
	return t.Hour() >= 15
}

func notWorktime(t time.Time) bool {
	if t.Weekday() == time.Saturday || t.Weekday() == time.Sunday {
		return true
	}
	// in utc (hate timezones)
	// [09:00:00, 18:00:00) msk
	if beforeWorktime(t) || afterWorktime(t) {
		return true
	}
	return false
}

func CheckBreachSLA(start, end time.Time, sla int) bool {
	start = start.UTC()

	slaDur := time.Hour * time.Duration(sla)

	for slaDur > 0 {
		if notWorktime(start) {
			datesDay := start.AddDate(0, 0, 1) // default = next day
			if beforeWorktime(start) {         // but in case it's now early in the morning...
				datesDay = start
			}
			start = time.Date(datesDay.Year(), datesDay.Month(), datesDay.Day(), 6, 0, 0, 0, time.UTC)
			continue
		}

		maxPossibleTime := time.Date(start.Year(), start.Month(), start.Day(), 14, 59, 59, 0, time.UTC)
		diff := maxPossibleTime.Sub(start)
		if diff < slaDur {
			slaDur -= diff
			start.Add(diff)
		} else {
			slaDur -= slaDur
			start.Add(slaDur)
		}
	}

	return end.Before(start)
}
