package pipeline

import (
	"time"
)

const (
	workingHoursStart = 6
	workingHoursEnd   = 15

	ddmmyyFormat = "02.01.2006"
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

func ComputeMaxDate(start time.Time, sla float32) time.Time {
	// SLA in hours
	// Convert to minutes
	deadline := start.UTC()
	slaInMinutes := sla * 60
	slaDur := time.Minute * time.Duration(slaInMinutes)

	for slaDur > 0 {
		if notWorkingHours(deadline) {
			datesDay := deadline.AddDate(0, 0, 1) // default = next day
			if beforeWorkingHours(deadline) {     // but in case it's now early in the morning...
				datesDay = deadline
			}
			deadline = time.Date(datesDay.Year(), datesDay.Month(), datesDay.Day(), 6, 0, 0, 0, time.UTC)
			continue
		}

		maxPossibleTime := time.Date(deadline.Year(), deadline.Month(), deadline.Day(), 15, 0, 0, 0, time.UTC)
		diff := maxPossibleTime.Sub(deadline)
		if diff < slaDur {
			deadline = deadline.Add(diff)
			slaDur -= diff
		} else {
			deadline = deadline.Add(slaDur)
			slaDur = 0
		}
	}

	return deadline
}

func CheckBreachSLA(start, current time.Time, sla int) bool {
	deadline := ComputeMaxDate(start, float32(sla))

	return current.UTC().After(deadline)
}

func ComputeDeadline(start time.Time, sla int) string {
	return ComputeMaxDate(start, float32(sla)).Format(ddmmyyFormat)
}
