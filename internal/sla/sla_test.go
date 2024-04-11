package sla

import (
	"testing"
	"time"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/hrgate"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

func Test_CheckBreachSLA(t *testing.T) {
	sla := NewSLAService(nil)

	type fields struct {
		Start   time.Time
		Current time.Time
		SLA     int
	}

	tests := []struct {
		name        string
		fields      fields
		wantedCheck bool
	}{
		{
			name: "bad sla",
			fields: fields{
				Start:   time.Date(2022, 0o1, 0o3, 6, 0, 0, 0, time.UTC),
				Current: time.Date(2022, 0o1, 0o3, 7, 0o1, 0, 0, time.UTC),
				SLA:     1,
			},
			wantedCheck: true,
		},
		{
			name: "ok sla",
			fields: fields{
				Start:   time.Date(2022, 0o1, 0o3, 6, 0, 0, 0, time.UTC),
				Current: time.Date(2022, 0o1, 0o3, 6, 30, 0, 0, time.UTC),
				SLA:     1,
			},
			wantedCheck: false,
		},
		{
			name: "ok sla next day",
			fields: fields{
				Start:   time.Date(2022, 0o1, 0o3, 14, 0, 0, 0, time.UTC),
				Current: time.Date(2022, 0o1, 0o4, 6, 30, 0, 0, time.UTC),
				SLA:     2,
			},
			wantedCheck: false,
		},
		{
			name: "ok sla (now before working hours)",
			fields: fields{
				Start:   time.Date(2022, 0o1, 0o3, 5, 0, 0, 0, time.UTC),
				Current: time.Date(2022, 0o1, 0o3, 6, 30, 0, 0, time.UTC),
				SLA:     1,
			},
			wantedCheck: false,
		},
		{
			name: "ok sla next day (now after working hours)",
			fields: fields{
				Start:   time.Date(2022, 0o1, 0o3, 18, 0, 0, 0, time.UTC),
				Current: time.Date(2022, 0o1, 0o4, 6, 30, 0, 0, time.UTC),
				SLA:     1,
			},
			wantedCheck: false,
		},
		{
			name: "ok sla after weekend",
			fields: fields{
				Start:   time.Date(2022, 0o1, 0o7, 18, 0, 0, 0, time.UTC),
				Current: time.Date(2022, 0o1, 10, 6, 30, 0, 0, time.UTC),
				SLA:     2,
			},
			wantedCheck: false,
		},
		{
			name: "ok sla next month",
			fields: fields{
				Start:   time.Date(2022, 0o1, 31, 18, 0, 0, 0, time.UTC),
				Current: time.Date(2022, 0o2, 0o1, 6, 30, 0, 0, time.UTC),
				SLA:     1,
			},
			wantedCheck: false,
		},
		{
			name: "ok sla next year",
			fields: fields{
				Start:   time.Date(2022, 12, 31, 18, 0, 0, 0, time.UTC),
				Current: time.Date(2023, 0o1, 0o2, 6, 30, 0, 0, time.UTC),
				SLA:     1,
			},
			wantedCheck: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if check := sla.CheckBreachSLA(tt.fields.Start, tt.fields.Current, tt.fields.SLA, nil); check != tt.wantedCheck {
				t.Errorf("check SLA returned unexpected result")
			}
		})
	}
}

func Test_ComputeDeadline(t *testing.T) {
	sla := NewSLAService(nil)

	type fields struct {
		Start time.Time
		SLA   int
	}

	tests := []struct {
		name   string
		fields fields
		wanted string
	}{
		{
			name: "this day",
			fields: fields{
				Start: time.Date(2022, 0o1, 0o3, 6, 0, 0, 0, time.UTC),
				SLA:   1,
			},
			wanted: "03.01.2022",
		},
		{
			name: "this day (now before working hours)",
			fields: fields{
				Start: time.Date(2022, 0o1, 0o3, 0, 0, 0, 0, time.UTC),
				SLA:   1,
			},
			wanted: "03.01.2022",
		},
		{
			name: "next day",
			fields: fields{
				Start: time.Date(2022, 0o1, 0o3, 6, 0, 0, 0, time.UTC),
				SLA:   10,
			},
			wanted: "04.01.2022",
		},
		{
			name: "after weekend",
			fields: fields{
				Start: time.Date(2022, 0o1, 0o7, 18, 0, 0, 0, time.UTC),
				SLA:   2,
			},
			wanted: "10.01.2022",
		},
		{
			name: "ok sla next month",
			fields: fields{
				Start: time.Date(2022, 0o1, 31, 18, 0, 0, 0, time.UTC),
				SLA:   1,
			},
			wanted: "01.02.2022",
		},
		{
			name: "ok sla next year",
			fields: fields{
				Start: time.Date(2022, 12, 31, 18, 0, 0, 0, time.UTC),
				SLA:   1,
			},
			wanted: "02.01.2023",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if deadline := sla.ComputeMaxDateFormatted(tt.fields.Start, tt.fields.SLA, nil); deadline != tt.wanted {
				t.Errorf("compute deadline returned unexpected result")
			}
		})
	}
}

func Test_getWorkWorkHoursBetweenDates(t *testing.T) {
	sla := NewSLAService(nil)

	type fields struct {
		from       time.Time
		to         time.Time
		slaInfoPtr *Info
	}

	tests := []struct {
		name          string
		fields        fields
		wantWorkHours int
	}{
		{
			name: "work days eq 2",
			fields: fields{
				from: time.Date(2022, 0o1, 0o3, 14, 0, 0, 0, time.UTC),
				to:   time.Date(2022, 0o1, 0o4, 6, 30, 0, 0, time.UTC),
			},
			wantWorkHours: 1,
		},
		{
			name: "work days eq 0",
			fields: fields{
				from: time.Date(2022, 0o7, 16, 14, 0, 0, 0, time.UTC),
				to:   time.Date(2022, 0o7, 17, 6, 30, 0, 0, time.UTC),
			},
			wantWorkHours: 0,
		},
		{
			name: "work days eq 8",
			fields: fields{
				from: time.Date(2022, 0o7, 16, 0, 0, 0, 0, time.UTC),
				to:   time.Date(2022, 0o7, 18, 15, 30, 0, 0, time.UTC),
			},
			wantWorkHours: 8,
		},
		{
			name: "work days eq 7 at preholidays days",
			fields: fields{
				from: time.Date(2022, 0o7, 16, 0, 0, 0, 0, time.UTC),
				to:   time.Date(2022, 0o7, 18, 15, 30, 0, 0, time.UTC),
				slaInfoPtr: &Info{
					CalendarDays: &hrgate.CalendarDays{
						CalendarMap: map[int64]hrgate.CalendarDayType{time.Date(2022, 0o7, 16, 0, 0, 0, 0, time.UTC).Unix(): hrgate.CalendarDayTypePreHoliday, time.Date(2022, 0o7, 17, 0, 0, 0, 0, time.UTC).Unix(): hrgate.CalendarDayTypePreHoliday, time.Date(2022, 0o7, 18, 0, 0, 0, 0, time.UTC).Unix(): hrgate.CalendarDayTypePreHoliday},
					},
				},
			},
			wantWorkHours: 7,
		},
		{
			name: "work hours eq 0 at holidays days",
			fields: fields{
				from: time.Date(2022, 0o7, 18, 0, 0, 0, 0, time.UTC),
				to:   time.Date(2022, 0o7, 21, 15, 30, 0, 0, time.UTC),
				slaInfoPtr: &Info{
					CalendarDays: &hrgate.CalendarDays{
						CalendarMap: map[int64]hrgate.CalendarDayType{time.Date(2022, 0o7, 18, 0, 0, 0, 0, time.UTC).Unix(): hrgate.CalendarDayTypeHoliday, time.Date(2022, 0o7, 19, 0, 0, 0, 0, time.UTC).Unix(): hrgate.CalendarDayTypeHoliday, time.Date(2022, 0o7, 20, 0, 0, 0, 0, time.UTC).Unix(): hrgate.CalendarDayTypeHoliday, time.Date(2022, 0o7, 21, 0, 0, 0, 0, time.UTC).Unix(): hrgate.CalendarDayTypeHoliday},
					},
				},
			},
			wantWorkHours: 0,
		},
		{
			name: "work days with preholidays and holidays days",
			fields: fields{
				from: time.Date(2022, 0o7, 19, 0, 0, 0, 0, time.UTC),
				to:   time.Date(2022, 0o7, 21, 15, 30, 0, 0, time.UTC),
				slaInfoPtr: &Info{
					CalendarDays: &hrgate.CalendarDays{
						CalendarMap: map[int64]hrgate.CalendarDayType{time.Date(2022, 0o7, 20, 0, 0, 0, 0, time.UTC).Unix(): hrgate.CalendarDayTypeHoliday, time.Date(2022, 0o7, 21, 0, 0, 0, 0, time.UTC).Unix(): hrgate.CalendarDayTypeHoliday, time.Date(2022, 0o7, 18, 0, 0, 0, 0, time.UTC).Unix(): hrgate.CalendarDayTypeHoliday, time.Date(2022, 0o7, 19, 0, 0, 0, 0, time.UTC).Unix(): hrgate.CalendarDayTypePreHoliday},
					},
				},
			},
			wantWorkHours: 7,
		},
		{
			name: "work days with work type 24/7 without holidays",
			fields: fields{
				from: time.Date(2022, 0o7, 1, 0, 0, 0, 0, time.UTC),
				to:   time.Date(2022, 0o7, 15, 0, 0, 0, 0, time.UTC),
				slaInfoPtr: &Info{
					StartWorkHourPtr: utils.GetAddressOfValue(-1),
					EndWorkHourPtr:   utils.GetAddressOfValue(25),
					Weekends:         []time.Weekday{},
				},
			},
			wantWorkHours: 336,
		},
		{
			name: "work days with work type 5/2 without holidays",
			fields: fields{
				from: time.Date(2022, 0o7, 1, 0, 0, 0, 0, time.UTC),
				to:   time.Date(2022, 0o7, 15, 0, 0, 0, 0, time.UTC),
				slaInfoPtr: &Info{
					StartWorkHourPtr: utils.GetAddressOfValue(6),
					EndWorkHourPtr:   utils.GetAddressOfValue(14),
					Weekends:         []time.Weekday{time.Saturday, time.Sunday},
				},
			},
			wantWorkHours: 80,
		},
		{
			name: "work days with work type 24/7 without holidays",
			fields: fields{
				from: time.Date(2022, 0o7, 1, 0, 0, 0, 0, time.UTC),
				to:   time.Date(2022, 0o7, 15, 0, 0, 0, 0, time.UTC),
				slaInfoPtr: &Info{
					StartWorkHourPtr: utils.GetAddressOfValue(6),
					EndWorkHourPtr:   utils.GetAddressOfValue(18),
					Weekends:         []time.Weekday{},
				},
			},
			wantWorkHours: 168,
		},
		{
			name: "work days with work type 24/7 without holidays",
			fields: fields{
				from: time.Date(2022, 0o7, 1, 0, 0, 0, 0, time.UTC),
				to:   time.Date(2022, 0o7, 15, 0, 0, 0, 0, time.UTC),
				slaInfoPtr: &Info{
					StartWorkHourPtr: utils.GetAddressOfValue(6),
					EndWorkHourPtr:   utils.GetAddressOfValue(14),
					Weekends:         []time.Weekday{time.Wednesday, time.Sunday},
				},
			},
			wantWorkHours: 80,
		},
		{
			name: "work days with work type 24/7 without holidays",
			fields: fields{
				from: time.Date(2022, 0o7, 1, 0, 0, 0, 0, time.UTC),
				to:   time.Date(2022, 0o7, 15, 0, 0, 0, 0, time.UTC),
				slaInfoPtr: &Info{
					StartWorkHourPtr: utils.GetAddressOfValue(6),
					EndWorkHourPtr:   utils.GetAddressOfValue(14),
					Weekends:         []time.Weekday{time.Monday},
					CalendarDays:     &hrgate.CalendarDays{CalendarMap: map[int64]hrgate.CalendarDayType{time.Date(2022, 0o7, 13, 0, 0, 0, 0, time.UTC).Unix(): hrgate.CalendarDayTypeHoliday, time.Date(2022, 0o7, 12, 0, 0, 0, 0, time.UTC).Unix(): hrgate.CalendarDayTypePreHoliday}},
				},
			},
			wantWorkHours: 87,
		},
		{
			name: "work days with work type 24/7 without holidays",
			fields: fields{
				from: time.Date(2022, 0o7, 1, 0, 0, 0, 0, time.UTC),
				to:   time.Date(2022, 0o7, 15, 0, 0, 0, 0, time.UTC),
				slaInfoPtr: &Info{
					StartWorkHourPtr: utils.GetAddressOfValue(6),
					EndWorkHourPtr:   utils.GetAddressOfValue(14),
					Weekends:         []time.Weekday{time.Tuesday},
					CalendarDays:     &hrgate.CalendarDays{CalendarMap: map[int64]hrgate.CalendarDayType{time.Date(2022, 0o7, 13, 0, 0, 0, 0, time.UTC).Unix(): hrgate.CalendarDayTypeHoliday, time.Date(2022, 0o7, 14, 0, 0, 0, 0, time.UTC).Unix(): hrgate.CalendarDayTypeHoliday}},
				},
			},
			wantWorkHours: 80,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotWorkHours := sla.GetWorkHoursBetweenDates(tt.fields.from, tt.fields.to, tt.fields.slaInfoPtr); gotWorkHours != tt.wantWorkHours {
				t.Errorf("getWorkHoursBetweenDates() = %v, want %v", gotWorkHours, tt.wantWorkHours)
			}
		})
	}
}
