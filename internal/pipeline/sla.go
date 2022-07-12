package pipeline

import "time"

const (
	workingHoursStart = 6
	workingHoursEnd   = 15
)

func getWorkWorkHoursBetweenDates(from, to time.Time) (workHours int) {
	if from.After(to) || from.Equal(to) {
		return 0
	}

	for from.Before(to) {
		if !notWorkingHours(from) {
			workHours++
		}

		from = from.Add(time.Hour * 1)
	}

	return workHours
}

func beforeWorkingHours(t time.Time) bool {
	return t.Hour() < workingHoursStart
}

func afterWorkingHours(t time.Time) bool {
	return t.Hour() >= workingHoursEnd
}

func notWorkingHours(t time.Time) bool {
	if t.Weekday() == time.Saturday || t.Weekday() == time.Sunday {
		return true
	}
	// in utc (hate timezones)
	// [09:00:00, 18:00:00) msk
	if beforeWorkingHours(t) || afterWorkingHours(t) {
		return true
	}
	return false
}

func CheckBreachSLA(limit, current time.Time, sla int) bool {
	limit = limit.UTC()
	current = current.UTC()

	slaDur := time.Hour * time.Duration(sla)

	for slaDur > 0 {
		if notWorkingHours(limit) {
			datesDay := limit.AddDate(0, 0, 1) // default = next day
			if beforeWorkingHours(limit) {     // but in case it's now early in the morning...
				datesDay = limit
			}
			limit = time.Date(datesDay.Year(), datesDay.Month(), datesDay.Day(), 6, 0, 0, 0, time.UTC)
			continue
		}

		maxPossibleTime := time.Date(limit.Year(), limit.Month(), limit.Day(), 15, 0, 0, 0, time.UTC)
		diff := maxPossibleTime.Sub(limit)
		if diff < slaDur {
			limit = limit.Add(diff)
			slaDur -= diff
		} else {
			limit = limit.Add(slaDur)
			slaDur = 0
		}
	}

	return current.After(limit)
}
