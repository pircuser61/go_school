package pipeline

import (
	"math"
	"time"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/hrgate"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

const (
	workingHoursStart = 6
	workingHoursEnd   = 15

	ddmmyyFormat = "02.01.2006"
)

func getWorkHoursBetweenDates(from, to time.Time, calendarDays *hrgate.CalendarDays, startWorkHour, endWorkHour int) (workHours int) {
	from = from.UTC()
	to = to.UTC()

	if from.After(to) || from.Equal(to) || to.Sub(from).Hours() < 1 {
		return 0
	}

	for from.Before(to) {
		if !notWorkingHours(from, calendarDays, startWorkHour, endWorkHour) {
			workHours++
		}

		from = from.Add(time.Hour * 1)
	}

	return workHours
}

func beforeWorkingHours(t time.Time, startHour int) bool {
	return t.Hour() < startHour
}

func afterWorkingHours(t time.Time, endHour int) bool {
	return t.Hour() >= endHour
}

func notWorkingHours(t time.Time, calendarDays *hrgate.CalendarDays, startWorkHour, endWorkHour int) bool {
	if t.Weekday() == time.Saturday || t.Weekday() == time.Sunday {
		return true
	}
	workDayType := calendarDays.GetDayType(t)
	if workDayType == hrgate.CalendarDayTypeHoliday {
		return true
	}

	if workDayType == hrgate.CalendarDayTypePreHoliday {
		endWorkHour = endWorkHour - 1
	}

	if beforeWorkingHours(t, startWorkHour) || afterWorkingHours(t, endWorkHour) {
		return true
	}
	return false
}

func ComputeMaxDate(start time.Time, sla float32, calendarDays *hrgate.CalendarDays, startWorkHour, endWorkHour int) time.Time {
	// SLA in hours
	// Convert to minutes
	deadline := start.UTC()
	slaInMinutes := sla * 60
	slaDur := time.Minute * time.Duration(slaInMinutes)

	for slaDur > 0 {
		if notWorkingHours(deadline, calendarDays, startWorkHour, endWorkHour) {
			datesDay := deadline.AddDate(0, 0, 1)                // default = next day
			if beforeWorkingHours(deadline, workingHoursStart) { // but in case it's now early in the morning...
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

func ComputeMeanTaskCompletionTime(taskIntervals []entity.TaskCompletionInterval, calendarDays hrgate.CalendarDays, startWorkHour, endWorkHour int) (
	result script.TaskSolveTime) {
	var taskIntervalsCnt = len(taskIntervals)

	var totalHours = 0
	for _, interval := range taskIntervals {
		totalHours += getWorkHoursBetweenDates(interval.StartedAt, interval.FinishedAt, &calendarDays, startWorkHour, endWorkHour)
	}

	return script.TaskSolveTime{
		MeanWorkHours: math.Ceil(float64(totalHours) / float64(taskIntervalsCnt)),
	}
}

func CheckBreachSLA(start, current time.Time, sla int, startWorkHour, endWorkHour int) bool {
	deadline := ComputeMaxDate(start, float32(sla), nil, startWorkHour, endWorkHour)

	return current.UTC().After(deadline)
}

func ComputeDeadline(start time.Time, sla int, calendarDays *hrgate.CalendarDays, startWorkHour, endWorkHour int) string {
	return ComputeMaxDate(start, float32(sla), calendarDays, startWorkHour, endWorkHour).Format(ddmmyyFormat)
}
