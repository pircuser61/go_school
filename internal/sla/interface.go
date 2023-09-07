package sla

import (
	"context"
	"time"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/hrgate"
	s "gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

type Service interface {
	GetSLAInfoPtr(ctx context.Context, GetSLAInfoDTO GetSLAInfoDTOStruct) (*SLAInfo, error)
	ComputeMaxDate(start time.Time, sla float32, slaInfoPtr *SLAInfo) time.Time
	ComputeMaxDateFormatted(start time.Time, sla int, slaInfoPtr *SLAInfo) string
	CheckBreachSLA(start, current time.Time, sla int, slaInfoPtr *SLAInfo) bool
	ComputeMeanTaskCompletionTime(intervals []entity.TaskCompletionInterval, days hrgate.CalendarDays) s.TaskSolveTime
	GetWorkHoursBetweenDates(from, to time.Time, slaInfoPtr *SLAInfo) (workHours int)
}
