package hrgate

import (
	"context"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

type ServiceInterface interface {
	GetCalendars(ctx context.Context, params *GetCalendarsParams) ([]Calendar, error)
	GetPrimaryRussianFederationCalendarOrFirst(ctx context.Context, params *GetCalendarsParams) (*Calendar, error)
	GetCalendarDays(ctx context.Context, params *GetCalendarDaysParams) (*CalendarDays, error)
	FillDefaultUnitID(ctx context.Context) error
	GetDefaultUnitID() string
	GetDefaultCalendarDaysForGivenTimeIntervals(ctx context.Context, taskTimeIntervals []entity.TaskCompletionInterval) (*CalendarDays, error)
}
