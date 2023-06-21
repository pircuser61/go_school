package pipeline

import (
	"context"
	"fmt"
	"math"
	"time"

	"golang.org/x/exp/slices"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/hrgate"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

const (
	workingHoursStart = 6
	workingHoursEnd   = 14

	ddmmyyFormat = "02.01.2006"
)

type WorkHourType string

type SLAInfo struct {
	CalendarDays     *hrgate.CalendarDays `json:"calendar_days"`
	StartWorkHourPtr *int                 `json:"start_work_hour"`
	EndWorkHourPtr   *int                 `json:"end_work_hour"`
	Weekends         []time.Weekday       `json:"weekends"`
}

type GetSLAInfoDTOStruct struct {
	Service                 *hrgate.Service
	TaskCompletionIntervals []entity.TaskCompletionInterval
	WorkType                WorkHourType
}

const (
	WorkTypeN125 WorkHourType = "12/5"

	WorkTypeN247 WorkHourType = "24/7"

	WorkTypeN85 WorkHourType = "8/5"
)

func (slaInfo *SLAInfo) GetCalendarDays() *hrgate.CalendarDays {
	if slaInfo == nil || slaInfo.CalendarDays == nil {
		return &hrgate.CalendarDays{CalendarMap: map[int64]hrgate.CalendarDayType{}}
	}

	return slaInfo.CalendarDays
}

func (slaInfo *SLAInfo) GetStartWorkHour() int {
	if slaInfo == nil || slaInfo.StartWorkHourPtr == nil {
		return workingHoursStart
	}

	return *slaInfo.StartWorkHourPtr
}

func (slaInfo *SLAInfo) GetEndWorkHour(t time.Time) int {
	workDayType := slaInfo.GetCalendarDays().GetDayType(t)

	if slaInfo == nil || slaInfo.EndWorkHourPtr == nil {
		if workDayType == hrgate.CalendarDayTypePreHoliday {
			return workingHoursEnd - 1
		}
		return workingHoursEnd
	}

	if workDayType == hrgate.CalendarDayTypePreHoliday {
		return *slaInfo.EndWorkHourPtr - 1
	}

	return *slaInfo.EndWorkHourPtr
}

func (slaInfo *SLAInfo) GetWeekends() []time.Weekday {
	if slaInfo == nil || slaInfo.Weekends == nil {
		return []time.Weekday{time.Saturday, time.Sunday}
	}

	return slaInfo.Weekends
}

func (t *WorkHourType) GetWorkingHours() (start, end int, err error) {
	if t == nil {
		return 0, 0, fmt.Errorf("work hour type is nil")
	}

	switch *t {
	case WorkTypeN125:
		return 6, 18, nil
	case WorkTypeN247:
		return -1, 25, nil
	case WorkTypeN85:
		return 6, 14, nil
	default:
		return 0, 0, fmt.Errorf("unknown work hour type: %s", string(*t))
	}
}

func (t *WorkHourType) GetWeekends() ([]time.Weekday, error) {
	if t == nil {
		return nil, fmt.Errorf("work hour type is nil")
	}

	switch *t {
	case WorkTypeN125:
		return []time.Weekday{time.Saturday, time.Sunday}, nil
	case WorkTypeN247:
		return []time.Weekday{}, nil
	case WorkTypeN85:
		return []time.Weekday{time.Saturday, time.Sunday}, nil
	default:
		return nil, fmt.Errorf("unknown work hour type: %s", string(*t))
	}
}

func (t *WorkHourType) GetNotUseCalendarDays() (bool, error) {
	if t == nil {
		return false, fmt.Errorf("work hour type is nil")
	}

	switch *t {
	case WorkTypeN125:
		return false, nil
	case WorkTypeN247:
		return true, nil
	case WorkTypeN85:
		return false, nil
	default:
		return false, fmt.Errorf("unknown work hour type: %s", string(*t))
	}
}

func (t *WorkHourType) GetTotalWorkHourPerDay() (int, error) {
	if t == nil {
		return 0, fmt.Errorf("work hour type is nil")
	}

	switch *t {
	case WorkTypeN125:
		return 12, nil
	case WorkTypeN247:
		return 24, nil
	case WorkTypeN85:
		return 8, nil
	default:
		return 0, fmt.Errorf("unknown work hour type: %s", string(*t))
	}
}

func (t *WorkHourType) GetTotalSLAInHours(slaInDays int) (int, error) {
	totalWorkHour, getTotalWorkHourErr := t.GetTotalWorkHourPerDay()

	if getTotalWorkHourErr != nil {
		return 0, getTotalWorkHourErr
	}

	return totalWorkHour * slaInDays, nil
}

func GetSLAInfoPtr(ctx context.Context, GetSLAInfoDTO GetSLAInfoDTOStruct) (*SLAInfo, error) {
	notUseCalendarDays, getNotUseErr := GetSLAInfoDTO.WorkType.GetNotUseCalendarDays()
	if getNotUseErr != nil {
		return nil, getNotUseErr
	}

	var calendarDays *hrgate.CalendarDays
	var getCalendarDaysErr error
	if !notUseCalendarDays {
		calendarDays, getCalendarDaysErr = GetSLAInfoDTO.Service.GetDefaultCalendarDaysForGivenTimeIntervals(ctx,
			GetSLAInfoDTO.TaskCompletionIntervals,
		)
		if getCalendarDaysErr != nil {
			return nil, getCalendarDaysErr
		}
	}

	startWorkHour, endWorkHour, getWorkingHoursErr := GetSLAInfoDTO.WorkType.GetWorkingHours()
	if getWorkingHoursErr != nil {
		return nil, getWorkingHoursErr
	}
	weekends, getWeekendsErr := GetSLAInfoDTO.WorkType.GetWeekends()
	if getWeekendsErr != nil {
		return nil, getWeekendsErr
	}

	return &SLAInfo{
		CalendarDays:     calendarDays,
		StartWorkHourPtr: &startWorkHour,
		EndWorkHourPtr:   &endWorkHour,
		Weekends:         weekends,
	}, nil
}

func getWorkHoursBetweenDates(from, to time.Time, slaInfoPtr *SLAInfo) (workHours int) {
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

func ComputeMaxDate(start time.Time, sla float32, slaInfoPtr *SLAInfo) time.Time {
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
			datesDay := deadline.AddDate(0, 0, 1)            // default = next day
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

func ComputeMeanTaskCompletionTime(taskIntervals []entity.TaskCompletionInterval, calendarDays hrgate.CalendarDays) (
	result script.TaskSolveTime) {
	var taskIntervalsCnt = len(taskIntervals)

	var totalHours = 0
	for _, interval := range taskIntervals {
		totalHours += getWorkHoursBetweenDates(interval.StartedAt, interval.FinishedAt, &SLAInfo{
			CalendarDays: &calendarDays,
		})
	}

	return script.TaskSolveTime{
		MeanWorkHours: math.Ceil(float64(totalHours) / float64(taskIntervalsCnt)),
	}
}

func CheckBreachSLA(start, current time.Time, sla int, slaInfoPtr *SLAInfo) bool {
	deadline := ComputeMaxDate(start, float32(sla), slaInfoPtr)

	return current.UTC().After(deadline)
}

func ComputeDeadline(start time.Time, sla int, slaInfoPtr *SLAInfo) string {
	return ComputeMaxDate(start, float32(sla), slaInfoPtr).Format(ddmmyyFormat)
}
