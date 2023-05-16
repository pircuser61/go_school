package pipeline

import (
	"testing"
	"time"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/hrgate"
)

func Test_CheckBreachSLA(t *testing.T) {
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
				Start:   time.Date(2022, 01, 03, 6, 0, 0, 0, time.UTC),
				Current: time.Date(2022, 01, 03, 7, 01, 0, 0, time.UTC),
				SLA:     1,
			},
			wantedCheck: true,
		},
		{
			name: "ok sla",
			fields: fields{
				Start:   time.Date(2022, 01, 03, 6, 0, 0, 0, time.UTC),
				Current: time.Date(2022, 01, 03, 6, 30, 0, 0, time.UTC),
				SLA:     1,
			},
			wantedCheck: false,
		},
		{
			name: "ok sla next day",
			fields: fields{
				Start:   time.Date(2022, 01, 03, 14, 0, 0, 0, time.UTC),
				Current: time.Date(2022, 01, 04, 6, 30, 0, 0, time.UTC),
				SLA:     2,
			},
			wantedCheck: false,
		},
		{
			name: "ok sla (now before working hours)",
			fields: fields{
				Start:   time.Date(2022, 01, 03, 5, 0, 0, 0, time.UTC),
				Current: time.Date(2022, 01, 03, 6, 30, 0, 0, time.UTC),
				SLA:     1,
			},
			wantedCheck: false,
		},
		{
			name: "ok sla next day (now after working hours)",
			fields: fields{
				Start:   time.Date(2022, 01, 03, 18, 0, 0, 0, time.UTC),
				Current: time.Date(2022, 01, 04, 6, 30, 0, 0, time.UTC),
				SLA:     1,
			},
			wantedCheck: false,
		},
		{
			name: "ok sla after weekend",
			fields: fields{
				Start:   time.Date(2022, 01, 07, 18, 0, 0, 0, time.UTC),
				Current: time.Date(2022, 01, 10, 6, 30, 0, 0, time.UTC),
				SLA:     2,
			},
			wantedCheck: false,
		},
		{
			name: "ok sla next month",
			fields: fields{
				Start:   time.Date(2022, 01, 31, 18, 0, 0, 0, time.UTC),
				Current: time.Date(2022, 02, 01, 6, 30, 0, 0, time.UTC),
				SLA:     1,
			},
			wantedCheck: false,
		},
		{
			name: "ok sla next year",
			fields: fields{
				Start:   time.Date(2022, 12, 31, 18, 0, 0, 0, time.UTC),
				Current: time.Date(2023, 01, 02, 6, 30, 0, 0, time.UTC),
				SLA:     1,
			},
			wantedCheck: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if check := CheckBreachSLA(tt.fields.Start, tt.fields.Current, tt.fields.SLA); check != tt.wantedCheck {
				t.Errorf("check SLA returned unexpected result")
			}
		})
	}
}

func Test_ComputeDeadline(t *testing.T) {
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
				Start: time.Date(2022, 01, 03, 6, 0, 0, 0, time.UTC),
				SLA:   1,
			},
			wanted: "03.01.2022",
		},
		{
			name: "this day (now before working hours)",
			fields: fields{
				Start: time.Date(2022, 01, 03, 0, 0, 0, 0, time.UTC),
				SLA:   1,
			},
			wanted: "03.01.2022",
		},
		{
			name: "next day",
			fields: fields{
				Start: time.Date(2022, 01, 03, 6, 0, 0, 0, time.UTC),
				SLA:   10,
			},
			wanted: "04.01.2022",
		},
		{
			name: "after weekend",
			fields: fields{
				Start: time.Date(2022, 01, 07, 18, 0, 0, 0, time.UTC),
				SLA:   2,
			},
			wanted: "10.01.2022",
		},
		{
			name: "ok sla next month",
			fields: fields{
				Start: time.Date(2022, 01, 31, 18, 0, 0, 0, time.UTC),
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
			if deadline := ComputeDeadline(tt.fields.Start, tt.fields.SLA); deadline != tt.wanted {
				t.Errorf("compute deadline returned unexpected result")
			}
		})
	}
}

func Test_getWorkWorkHoursBetweenDates(t *testing.T) {
	type fields struct {
		from         time.Time
		to           time.Time
		calendarDays *hrgate.CalendarDays
	}
	tests := []struct {
		name          string
		fields        fields
		wantWorkHours int
	}{
		{
			name: "work days eq 2",
			fields: fields{
				from:         time.Date(2022, 01, 03, 14, 0, 0, 0, time.UTC),
				to:           time.Date(2022, 01, 04, 6, 30, 0, 0, time.UTC),
				calendarDays: nil,
			},
			wantWorkHours: 2,
		},
		{
			name: "work days eq 0",
			fields: fields{
				from:         time.Date(2022, 07, 16, 14, 0, 0, 0, time.UTC),
				to:           time.Date(2022, 07, 17, 6, 30, 0, 0, time.UTC),
				calendarDays: nil,
			},
			wantWorkHours: 0,
		},
		{
			name: "work days eq 9",
			fields: fields{
				from:         time.Date(2022, 07, 16, 0, 0, 0, 0, time.UTC),
				to:           time.Date(2022, 07, 18, 15, 30, 0, 0, time.UTC),
				calendarDays: nil,
			},
			wantWorkHours: 9,
		},
		{
			name: "work days eq 9 at preholidays days",
			fields: fields{
				from: time.Date(2022, 07, 16, 0, 0, 0, 0, time.UTC),
				to:   time.Date(2022, 07, 18, 15, 30, 0, 0, time.UTC),
				calendarDays: &hrgate.CalendarDays{
					Holidays: nil,
					PreHolidays: []int64{time.Date(2022, 07, 16, 0, 0, 0, 0, time.UTC).Unix(),
						time.Date(2022, 07, 17, 0, 0, 0, 0, time.UTC).Unix(),
						time.Date(2022, 07, 18, 0, 0, 0, 0, time.UTC).Unix(),
					},
					WorkDay: nil,
				},
			},
			wantWorkHours: 8,
		},
		{
			name: "work days eq 0 at holidays days",
			fields: fields{
				from: time.Date(2022, 07, 18, 0, 0, 0, 0, time.UTC),
				to:   time.Date(2022, 07, 21, 15, 30, 0, 0, time.UTC),
				calendarDays: &hrgate.CalendarDays{
					Holidays: []int64{time.Date(2022, 07, 18, 0, 0, 0, 0, time.UTC).Unix(),
						time.Date(2022, 07, 19, 0, 0, 0, 0, time.UTC).Unix(),
						time.Date(2022, 07, 20, 0, 0, 0, 0, time.UTC).Unix(),
						time.Date(2022, 07, 21, 0, 0, 0, 0, time.UTC).Unix(),
					},
				},
			},
			wantWorkHours: 0,
		},
		{
			name: "work days eq 0 at holidays, preholidays days",
			fields: fields{
				from: time.Date(2022, 07, 18, 0, 0, 0, 0, time.UTC),
				to:   time.Date(2022, 07, 21, 15, 30, 0, 0, time.UTC),
				calendarDays: &hrgate.CalendarDays{
					Holidays: []int64{
						time.Date(2022, 07, 20, 0, 0, 0, 0, time.UTC).Unix(),
						time.Date(2022, 07, 21, 0, 0, 0, 0, time.UTC).Unix(),
					},
					PreHolidays: []int64{
						time.Date(2022, 07, 18, 0, 0, 0, 0, time.UTC).Unix(),
						time.Date(2022, 07, 19, 0, 0, 0, 0, time.UTC).Unix(),
					},
				},
			},
			wantWorkHours: 16,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotWorkHours := getWorkHoursBetweenDates(tt.fields.from, tt.fields.to, tt.fields.calendarDays); gotWorkHours != tt.wantWorkHours {
				t.Errorf("getWorkHoursBetweenDates() = %v, want %v", gotWorkHours, tt.wantWorkHours)
			}
		})
	}
}
