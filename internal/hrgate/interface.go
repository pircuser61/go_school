package hrgate

import (
	c "context"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

type ServiceInterface interface {
	GetCalendars(ctx c.Context, params *GetCalendarsParams) ([]Calendar, error)
	GetPrimaryRussianFederationCalendarOrFirst(ctx c.Context, params *GetCalendarsParams) (*Calendar, error)
	GetCalendarDays(ctx c.Context, params *GetCalendarDaysParams) (*CalendarDays, error)
	FillDefaultUnitID(ctx c.Context) error
	GetDefaultUnitID() string
	GetDefaultCalendarDaysForGivenTimeIntervals(ctx c.Context, intervals []entity.TaskCompletionInterval) (*CalendarDays, error)
	Ping(ctx c.Context) error
}
