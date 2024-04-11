package sla

import (
	"context"
	"math"
	"time"

	"golang.org/x/exp/slices"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/hrgate"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

type service struct {
	HrGate hrgate.ServiceInterface
}

func NewSLAService(hrGate hrgate.ServiceInterface) Service {
	return &service{
		HrGate: hrGate,
	}
}

func (s *service) GetSLAInfoPtr(ctx context.Context, dto InfoDTO) (*Info, error) {
	startWorkHour, endWorkHour, getWorkingHoursErr := dto.WorkType.GetWorkingHours()
	if getWorkingHoursErr != nil {
		return nil, getWorkingHoursErr
	}

	weekends, getWeekendsErr := dto.WorkType.GetWeekends()
	if getWeekendsErr != nil {
		return nil, getWeekendsErr
	}

	if s.HrGate == nil {
		return &Info{
			CalendarDays:     &hrgate.CalendarDays{CalendarMap: map[int64]hrgate.CalendarDayType{}},
			StartWorkHourPtr: &startWorkHour,
			EndWorkHourPtr:   &endWorkHour,
			Weekends:         weekends,
		}, nil
	}

	notUseCalendarDays, getNotUseErr := dto.WorkType.GetNotUseCalendarDays()
	if getNotUseErr != nil {
		return nil, getNotUseErr
	}

	var (
		calendarDays       *hrgate.CalendarDays
		getCalendarDaysErr error
	)

	if !notUseCalendarDays {
		calendarDays, getCalendarDaysErr = s.HrGate.GetDefaultCalendarDaysForGivenTimeIntervals(ctx,
			dto.TaskCompletionIntervals,
		)
		if getCalendarDaysErr != nil {
			return nil, getCalendarDaysErr
		}
	}

	startWorkHour, endWorkHour, getWorkingHoursErr = dto.WorkType.GetWorkingHours()
	if getWorkingHoursErr != nil {
		return nil, getWorkingHoursErr
	}

	weekends, getWeekendsErr = dto.WorkType.GetWeekends()
	if getWeekendsErr != nil {
		return nil, getWeekendsErr
	}

	return &Info{
		CalendarDays:     calendarDays,
		StartWorkHourPtr: &startWorkHour,
		EndWorkHourPtr:   &endWorkHour,
		Weekends:         weekends,
	}, nil
}

func (s *service) ComputeMaxDate(start time.Time, sla float32, slaInfoPtr *Info) time.Time {
	// SLA in hours
	// Convert to minutes
	deadline := start.UTC()
	slaInMinutes := sla * 60
	slaDur := time.Minute * time.Duration(slaInMinutes)

	for slaDur > 0 {
		calendarDays, startWorkHour, endWorkHour, weekends := slaInfoPtr.GetCalendarDays(),
			slaInfoPtr.GetStartWorkHour(),
			slaInfoPtr.GetEndWorkHour(deadline),
			slaInfoPtr.GetWeekends()
		if notWorkingHours(deadline, calendarDays, startWorkHour, endWorkHour, weekends) {
			datesDay := deadline.AddDate(0, 0, 1) // default = next day

			if beforeWorkingHours(deadline, startWorkHour) { // but in case it's now early in the morning...
				datesDay = deadline
			}

			deadline = time.Date(datesDay.Year(), datesDay.Month(), datesDay.Day(), startWorkHour, 0, 0, 0, time.UTC)

			continue
		}

		maxPossibleTime := time.Date(deadline.Year(), deadline.Month(), deadline.Day(), endWorkHour, 0, 0, 0, time.UTC)

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

func (s *service) CheckBreachSLA(start, current time.Time, sla int, slaInfoPtr *Info) bool {
	deadline := s.ComputeMaxDate(start, float32(sla), slaInfoPtr)

	return current.UTC().After(deadline)
}

func (s *service) ComputeMaxDateFormatted(start time.Time, sla int, slaInfoPtr *Info) string {
	return s.ComputeMaxDate(start, float32(sla), slaInfoPtr).Format(ddmmyyFormat)
}

func (s *service) ComputeMeanTaskCompletionTime(intervals []entity.TaskCompletionInterval, calendarDays hrgate.CalendarDays) (
	result script.TaskSolveTime,
) {
	taskIntervalsCnt := len(intervals)

	totalHours := 0
	for _, interval := range intervals {
		totalHours += s.GetWorkHoursBetweenDates(interval.StartedAt, interval.FinishedAt, &Info{
			CalendarDays: &calendarDays,
		})
	}

	return script.TaskSolveTime{
		MeanWorkHours: math.Ceil(float64(totalHours) / float64(taskIntervalsCnt)),
	}
}

func (s *service) GetWorkHoursBetweenDates(from, to time.Time, slaInfoPtr *Info) (workHours int) {
	from = from.UTC()
	to = to.UTC()

	if from.After(to) || from.Equal(to) || to.Sub(from).Hours() < 1 {
		return 0
	}

	for from.Before(to) {
		calendarDays, startWorkHour, endWorkHour, weekends := slaInfoPtr.GetCalendarDays(),
			slaInfoPtr.GetStartWorkHour(),
			slaInfoPtr.GetEndWorkHour(from),
			slaInfoPtr.GetWeekends()
		if !notWorkingHours(from, calendarDays, startWorkHour, endWorkHour, weekends) {
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

func notWorkingHours(t time.Time, calendarDays *hrgate.CalendarDays, startWorkHour, endWorkHour int, weekends []time.Weekday) bool {
	if slices.Contains(weekends, t.Weekday()) {
		return true
	}

	workDayType := calendarDays.GetDayType(t)

	if workDayType == hrgate.CalendarDayTypeHoliday {
		return true
	}

	if beforeWorkingHours(t, startWorkHour) || afterWorkingHours(t, endWorkHour) {
		return true
	}

	return false
}
