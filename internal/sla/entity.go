package sla

import (
	"fmt"
	"time"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/hrgate"
)

const (
	workingHoursStart = 6
	workingHoursEnd   = 14

	ddmmyyFormat = "02.01.2006"
)

type WorkHourType string

type Info struct {
	CalendarDays     *hrgate.CalendarDays `json:"calendar_days"`
	StartWorkHourPtr *int                 `json:"start_work_hour"`
	EndWorkHourPtr   *int                 `json:"end_work_hour"`
	Weekends         []time.Weekday       `json:"weekends"`
}

type InfoDTO struct {
	TaskCompletionIntervals []entity.TaskCompletionInterval
	WorkType                WorkHourType
}

const (
	WorkTypeN125 WorkHourType = "12/5"

	WorkTypeN247 WorkHourType = "24/7"

	WorkTypeN85 WorkHourType = "8/5"
)

func (slaInfo *Info) GetCalendarDays() *hrgate.CalendarDays {
	if slaInfo == nil || slaInfo.CalendarDays == nil {
		return &hrgate.CalendarDays{CalendarMap: map[int64]hrgate.CalendarDayType{}}
	}

	return slaInfo.CalendarDays
}

func (slaInfo *Info) GetStartWorkHour() int {
	if slaInfo == nil || slaInfo.StartWorkHourPtr == nil {
		return workingHoursStart
	}

	return *slaInfo.StartWorkHourPtr
}

func (slaInfo *Info) GetEndWorkHour(t time.Time) int {
	workDayType, found := slaInfo.GetCalendarDays().GetDayType(t)

	if slaInfo == nil || slaInfo.EndWorkHourPtr == nil {
		if found && workDayType == hrgate.CalendarDayTypePreHoliday {
			return workingHoursEnd - 1
		}

		return workingHoursEnd
	}

	if found && workDayType == hrgate.CalendarDayTypePreHoliday {
		return *slaInfo.EndWorkHourPtr - 1
	}

	return *slaInfo.EndWorkHourPtr
}

func (slaInfo *Info) GetWeekends() []time.Weekday {
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
