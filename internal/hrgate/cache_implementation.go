package hrgate

import (
	"context"
	"encoding/json"
	"fmt"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	cachekit "gitlab.services.mts.ru/jocasta/cache-kit"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

const (
	calendarsKeyPrefix    = "calendar:"
	calendarDaysKeyPrefix = "calendarDays:"
)

type ServiceWithCache struct {
	Cache  cachekit.Cache
	HRGate ServiceInterface
}

func (s *ServiceWithCache) GetCalendars(ctx context.Context, params *GetCalendarsParams) ([]Calendar, error) {
	log := logger.CreateLogger(nil)

	key, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %s", err)
	}

	keyForCache := calendarsKeyPrefix + string(key)

	valueFromCache, err := s.Cache.GetValue(ctx, keyForCache)
	if err == nil {
		calendars, ok := valueFromCache.([]Calendar)
		if ok {
			return calendars, nil
		}

		err = s.Cache.DeleteValue(ctx, keyForCache)
		if err != nil {
			log.WithError(err).Error("can't delete key from cache")
		}
	}

	calendar, err := s.HRGate.GetCalendars(ctx, params)
	if err != nil {
		return nil, err
	}

	err = s.Cache.SetValue(ctx, keyForCache, calendar)
	if err != nil {
		log.WithError(err).Error("can't send data to cache")
	}

	return calendar, nil
}

func (s *ServiceWithCache) GetCalendarDays(ctx context.Context, params *GetCalendarDaysParams) (*CalendarDays, error) {
	key, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %s", err)
	}

	keyForCache := calendarDaysKeyPrefix + string(key)

	valueFromCache, err := s.Cache.GetValue(ctx, keyForCache)
	if err == nil {
		calendarDays, ok := valueFromCache.(*CalendarDays)
		if !ok {
			return nil, fmt.Errorf("failed to cast value from cache to type CalendarDays")
		}

		return calendarDays, nil
	}

	calendarDays, err := s.HRGate.GetCalendarDays(ctx, params)
	if err != nil {
		return nil, err
	}

	err = s.Cache.SetValue(ctx, keyForCache, calendarDays)
	if err != nil {
		return nil, fmt.Errorf("can't set calendarDays to cache: %s", err)
	}

	return calendarDays, nil
}

// TODO скопировать с сервиса
func (s *ServiceWithCache) GetPrimaryRussianFederationCalendarOrFirst(ctx context.Context, params *GetCalendarsParams) (*Calendar, error) {
	return s.HRGate.GetPrimaryRussianFederationCalendarOrFirst(ctx, params)
}

func (s *ServiceWithCache) FillDefaultUnitID(ctx context.Context) error {
	return s.HRGate.FillDefaultUnitID(ctx)
}

func (s *ServiceWithCache) GetDefaultUnitID() string {
	return s.HRGate.GetDefaultUnitID()
}

// TODO удалить из интерфейса
func (s *ServiceWithCache) GetDefaultCalendar(ctx context.Context) (*Calendar, error) {
	return s.HRGate.GetDefaultCalendar(ctx)
}

// TODO скопировать с сервиса
func (s *ServiceWithCache) GetDefaultCalendarDaysForGivenTimeIntervals(
	ctx context.Context,
	taskTimeIntervals []entity.TaskCompletionInterval,
) (*CalendarDays, error) {
	return s.HRGate.GetDefaultCalendarDaysForGivenTimeIntervals(ctx, taskTimeIntervals)
}
