package cache

import (
	c "context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/hrgate"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/hrgate/nocache"
	"go.opencensus.io/trace"

	openapi_types "github.com/deepmap/oapi-codegen/pkg/types"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	cachekit "gitlab.services.mts.ru/jocasta/cache-kit"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

const (
	calendarsKeyPrefix    = "calendar:"
	calendarDaysKeyPrefix = "calendarDays:"
)

type service struct {
	Cache  cachekit.Cache
	HRGate hrgate.Service
}

func NewService(cfg *hrgate.Config, ssoS *sso.Service, m metrics.Metrics) (hrgate.Service, error) {
	srv, err := nocache.NewService(cfg, ssoS, m)
	if err != nil {
		return nil, err
	}

	cache, cacheErr := cachekit.CreateCache(cachekit.Config(cfg.Cache))
	if cacheErr != nil {
		return nil, cacheErr
	}

	return &service{
		HRGate: srv,
		Cache:  cache,
	}, nil
}

func (s *service) GetCalendars(ctx c.Context, params *hrgate.GetCalendarsParams) ([]hrgate.Calendar, error) {
	ctx, span := trace.StartSpan(ctx, "hrgate.get_calendars(cached)")
	defer span.End()

	log := logger.GetLogger(ctx)

	var keyForCache string

	key, err := json.Marshal(params)
	if err == nil { //nolint:nestif //так нужно
		keyForCache = calendarsKeyPrefix + string(key)

		valueFromCache, err := s.Cache.GetValue(ctx, keyForCache) //nolint:govet //ничего страшного
		if err == nil {
			calendars, ok := valueFromCache.(string)
			if ok {
				var data []hrgate.Calendar

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
	}

	calendar, err := s.HRGate.GetCalendars(ctx, params)
	if err != nil {
		return nil, err
	}

	calendarData, err := json.Marshal(calendar)
	if err == nil && keyForCache != "" {
		err = s.Cache.SetValue(ctx, keyForCache, string(calendarData))
		if err != nil {
			log.WithError(err).Error("can't send data to cache")
		}
	}

	return calendar, nil
}

func (s *service) Ping(ctx c.Context) error {
	return s.HRGate.Ping(ctx)
}

func (s *service) GetCalendarDays(ctx c.Context, params *hrgate.GetCalendarDaysParams) (*hrgate.CalendarDays, error) {
	ctx, span := trace.StartSpan(ctx, "hrgate.get_calendar_days(cached)")
	defer span.End()

	log := logger.GetLogger(ctx)

	var keyForCache string

	key, err := json.Marshal(params)
	if err == nil { //nolint:nestif //так нужно
		keyForCache = calendarDaysKeyPrefix + string(key)

		valueFromCache, err := s.Cache.GetValue(ctx, keyForCache) //nolint:govet //ничего страшного
		if err == nil {
			calendarDays, ok := valueFromCache.(string)
			if ok {
				var data *hrgate.CalendarDays

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
	}

	calendarDays, err := s.HRGate.GetCalendarDays(ctx, params)
	if err != nil {
		return nil, err
	}

	calendarDaysData, err := json.Marshal(calendarDays)
	if err == nil && keyForCache != "" {
		err = s.Cache.SetValue(ctx, keyForCache, string(calendarDaysData))
		if err != nil {
			return nil, fmt.Errorf("can't set calendarDays to cache: %s", err)
		}
	}

	return calendarDays, nil
}

func (s *service) GetPrimaryRussianFederationCalendarOrFirst(ctx c.Context, params *hrgate.GetCalendarsParams) (*hrgate.Calendar, error) {
	ctx, span := trace.StartSpan(ctx, "hrgate.get_primary_calendar_or_first(cached)")
	defer span.End()

	calendars, getCalendarsErr := s.GetCalendars(ctx, params)

	if getCalendarsErr != nil {
		return nil, getCalendarsErr
	}

	for calendarIdx := range calendars {
		calendar := calendars[calendarIdx]

		if calendar.Primary != nil && *calendar.Primary && calendar.HolidayCalendar == nocache.RussianFederation {
			return &calendar, nil
		}
	}

	return &calendars[0], nil
}

func (s *service) FillDefaultUnitID(ctx c.Context) error {
	return s.HRGate.FillDefaultUnitID(ctx)
}

func (s *service) GetDefaultUnitID() string {
	return s.HRGate.GetDefaultUnitID()
}

// nolint:dupl //так нужно!
func (s *service) GetDefaultCalendarDaysForGivenTimeIntervals(
	ctx c.Context,
	taskTimeIntervals []entity.TaskCompletionInterval,
) (*hrgate.CalendarDays, error) {
	ctx, span := trace.StartSpan(ctx, "hrgate.get_default_calendar_days_for_given_time_intervals(cached)")
	defer span.End()

	unitID := s.GetDefaultUnitID()

	calendar, getCalendarsErr := s.GetPrimaryRussianFederationCalendarOrFirst(ctx, &hrgate.GetCalendarsParams{
		UnitIDs: &hrgate.UnitIDs{unitID},
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

	calendarDays, getCalendarDaysErr := s.GetCalendarDays(ctx, &hrgate.GetCalendarDaysParams{
		QueryFilters: &hrgate.QueryFilters{
			WithDeleted: utils.GetAddressOfValue(false),
			Limit: utils.GetAddressOfValue(int(math.Ceil(utils.GetDateUnitNumBetweenDates(minIntervalTime.StartedAt,
				maxIntervalTime.FinishedAt, utils.Day)))),
		},
		Calendar: &hrgate.IDsList{string(calendar.Id)},
		DateFrom: &openapi_types.Date{Time: minIntervalTime.StartedAt},
		DateTo:   &openapi_types.Date{Time: maxIntervalTime.FinishedAt},
	})
	if getCalendarDaysErr != nil {
		return nil, err
	}

	return calendarDays, nil
}

func (s *service) GetComplexAssignmentsV2(ctx c.Context, logins []string) ([]entity.AssignmentsV2, error) {
	ctx, span := trace.StartSpan(ctx, "hrgate.get_complex_assignmentsV2")
	defer span.End()

	result, err := s.HRGate.GetComplexAssignmentsV2(ctx, logins)
	if err != nil {
		return nil, err
	}

	return result, nil
}
