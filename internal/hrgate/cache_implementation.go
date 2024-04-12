package hrgate

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"go.opencensus.io/trace"

	openapi_types "github.com/deepmap/oapi-codegen/pkg/types"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	cachekit "gitlab.services.mts.ru/jocasta/cache-kit"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
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
	ctx, span := trace.StartSpan(ctx, "hrgate.get_calendars(cached)")
	defer span.End()

	log := logger.GetLogger(ctx)

	key, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %s", err)
	}

	keyForCache := calendarsKeyPrefix + string(key)

	valueFromCache, err := s.Cache.GetValue(ctx, keyForCache)
	if err == nil {
		calendars, ok := valueFromCache.(string)
		if ok {
			var data []Calendar

			unmErr := json.Unmarshal([]byte(calendars), &data)
			if unmErr == nil {
				log.Info("got calendars from cache")

				return data, nil
			}
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

	calendarData, err := json.Marshal(calendar)
	if err == nil {
		err = s.Cache.SetValue(ctx, keyForCache, string(calendarData))
		if err != nil {
			log.WithError(err).Error("can't send data to cache")
		}
	}

	return calendar, nil
}

func (s *ServiceWithCache) GetCalendarDays(ctx context.Context, params *GetCalendarDaysParams) (*CalendarDays, error) {
	ctx, span := trace.StartSpan(ctx, "hrgate.get_calendar_days(cached)")
	defer span.End()

	log := logger.GetLogger(ctx)

	key, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %s", err)
	}

	keyForCache := calendarDaysKeyPrefix + string(key)

	valueFromCache, err := s.Cache.GetValue(ctx, keyForCache)
	if err == nil {
		calendarDays, ok := valueFromCache.(string)
		if ok {
			var data *CalendarDays

			unmErr := json.Unmarshal([]byte(calendarDays), &data)
			if unmErr == nil {
				log.Info("got calendarDays from cache")

				return data, nil
			}
		}

		err = s.Cache.DeleteValue(ctx, keyForCache)
		if err != nil {
			log.WithError(err).Error("can't delete key from cache")
		}
	}

	calendarDays, err := s.HRGate.GetCalendarDays(ctx, params)
	if err != nil {
		return nil, err
	}

	calendarDaysData, err := json.Marshal(calendarDays)
	if err == nil {
		err = s.Cache.SetValue(ctx, keyForCache, string(calendarDaysData))
		if err != nil {
			return nil, fmt.Errorf("can't set calendarDays to cache: %s", err)
		}
	}

	return calendarDays, nil
}

func (s *ServiceWithCache) GetPrimaryRussianFederationCalendarOrFirst(ctx context.Context, params *GetCalendarsParams) (*Calendar, error) {
	ctx, span := trace.StartSpan(ctx, "hrgate.get_primary_calendar_or_first(cached)")
	defer span.End()

	calendars, getCalendarsErr := s.GetCalendars(ctx, params)

	if getCalendarsErr != nil {
		return nil, getCalendarsErr
	}

	for calendarIdx := range calendars {
		calendar := calendars[calendarIdx]

		if calendar.Primary != nil && *calendar.Primary && calendar.HolidayCalendar == RussianFederation {
			return &calendar, nil
		}
	}

	return &calendars[0], nil
}

func (s *ServiceWithCache) FillDefaultUnitID(ctx context.Context) error {
	return s.HRGate.FillDefaultUnitID(ctx)
}

func (s *ServiceWithCache) GetDefaultUnitID() string {
	return s.HRGate.GetDefaultUnitID()
}

func (s *ServiceWithCache) GetDefaultCalendarDaysForGivenTimeIntervals(
	ctx context.Context,
	taskTimeIntervals []entity.TaskCompletionInterval,
) (*CalendarDays, error) {
	ctx, span := trace.StartSpan(ctx, "hrgate.get_default_calendar_days_for_given_time_intervals(cached)")
	defer span.End()

	unitID := s.GetDefaultUnitID()

	calendar, getCalendarsErr := s.GetPrimaryRussianFederationCalendarOrFirst(ctx, &GetCalendarsParams{
		UnitIDs: &UnitIDs{unitID},
	})

	if getCalendarsErr != nil {
		return nil, getCalendarsErr
	}

	minIntervalTime, err := utils.FindMin(taskTimeIntervals, func(a, b entity.TaskCompletionInterval) bool {
		return a.StartedAt.Unix() < b.StartedAt.Unix()
	})
	if err != nil {
		return nil, err
	}

	minIntervalTime.StartedAt = minIntervalTime.StartedAt.Add(-time.Hour * 24 * 7)

	maxIntervalTime, err := utils.FindMax(taskTimeIntervals, func(a, b entity.TaskCompletionInterval) bool {
		return a.StartedAt.Unix() < b.StartedAt.Unix()
	})
	if err != nil {
		return nil, err
	}

	maxIntervalTime.FinishedAt = minIntervalTime.FinishedAt.Add(time.Hour * 24 * 7) // just taking more time

	calendarDays, getCalendarDaysErr := s.GetCalendarDays(ctx, &GetCalendarDaysParams{
		QueryFilters: &QueryFilters{
			WithDeleted: utils.GetAddressOfValue(false),
			Limit: utils.GetAddressOfValue(int(math.Ceil(utils.GetDateUnitNumBetweenDates(minIntervalTime.StartedAt,
				maxIntervalTime.FinishedAt, utils.Day)))),
		},
		Calendar: &IDsList{string(calendar.Id)},
		DateFrom: &openapi_types.Date{Time: minIntervalTime.StartedAt},
		DateTo:   &openapi_types.Date{Time: maxIntervalTime.FinishedAt},
	})
	if getCalendarDaysErr != nil {
		return nil, err
	}

	return calendarDays, nil
}
