package pipeline

import (
	"testing"
	"time"
)

func Test_CheckBreachSLA(t *testing.T) {
	type fields struct {
		Limit   time.Time
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
				Limit:   time.Date(2022, 01, 03, 6, 0, 0, 0, time.UTC),
				Current: time.Date(2022, 01, 03, 7, 01, 0, 0, time.UTC),
				SLA:     1,
			},
			wantedCheck: true,
		},
		{
			name: "ok sla",
			fields: fields{
				Limit:   time.Date(2022, 01, 03, 6, 0, 0, 0, time.UTC),
				Current: time.Date(2022, 01, 03, 6, 30, 0, 0, time.UTC),
				SLA:     1,
			},
			wantedCheck: false,
		},
		{
			name: "ok sla next day",
			fields: fields{
				Limit:   time.Date(2022, 01, 03, 14, 0, 0, 0, time.UTC),
				Current: time.Date(2022, 01, 04, 6, 30, 0, 0, time.UTC),
				SLA:     2,
			},
			wantedCheck: false,
		},
		{
			name: "ok sla (now before worktime)",
			fields: fields{
				Limit:   time.Date(2022, 01, 03, 5, 0, 0, 0, time.UTC),
				Current: time.Date(2022, 01, 03, 6, 30, 0, 0, time.UTC),
				SLA:     1,
			},
			wantedCheck: false,
		},
		{
			name: "ok sla next day (now after worktime)",
			fields: fields{
				Limit:   time.Date(2022, 01, 03, 18, 0, 0, 0, time.UTC),
				Current: time.Date(2022, 01, 04, 6, 30, 0, 0, time.UTC),
				SLA:     1,
			},
			wantedCheck: false,
		},
		{
			name: "ok sla after weekend",
			fields: fields{
				Limit:   time.Date(2022, 01, 07, 18, 0, 0, 0, time.UTC),
				Current: time.Date(2022, 01, 10, 6, 30, 0, 0, time.UTC),
				SLA:     2,
			},
			wantedCheck: false,
		},
		{
			name: "ok sla next month",
			fields: fields{
				Limit:   time.Date(2022, 01, 31, 18, 0, 0, 0, time.UTC),
				Current: time.Date(2022, 02, 01, 6, 30, 0, 0, time.UTC),
				SLA:     1,
			},
			wantedCheck: false,
		},
		{
			name: "ok sla next year",
			fields: fields{
				Limit:   time.Date(2022, 12, 31, 18, 0, 0, 0, time.UTC),
				Current: time.Date(2023, 01, 02, 6, 30, 0, 0, time.UTC),
				SLA:     1,
			},
			wantedCheck: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if check := CheckBreachSLA(tt.fields.Limit, tt.fields.Current, tt.fields.SLA); check != tt.wantedCheck {
				t.Errorf("check SLA returned unexpected result")
			}
		})
	}
}

func Test_getWorkWorkHoursBetweenDates(t *testing.T) {
	type fields struct {
		from time.Time
		to   time.Time
	}
	tests := []struct {
		name          string
		fields        fields
		wantWorkHours int
	}{
		{
			name: "work days eq 2",
			fields: fields{
				from: time.Date(2022, 01, 03, 14, 0, 0, 0, time.UTC),
				to:   time.Date(2022, 01, 04, 6, 30, 0, 0, time.UTC),
			},
			wantWorkHours: 2,
		},
		{
			name: "work days eq 0",
			fields: fields{
				from: time.Date(2022, 07, 16, 14, 0, 0, 0, time.UTC),
				to:   time.Date(2022, 07, 17, 6, 30, 0, 0, time.UTC),
			},
			wantWorkHours: 0,
		},
		{
			name: "work days eq 9",
			fields: fields{
				from: time.Date(2022, 07, 16, 0, 0, 0, 0, time.UTC),
				to:   time.Date(2022, 07, 18, 15, 30, 0, 0, time.UTC),
			},
			wantWorkHours: 9,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotWorkHours := getWorkWorkHoursBetweenDates(tt.fields.from, tt.fields.to); gotWorkHours != tt.wantWorkHours {
				t.Errorf("getWorkWorkHoursBetweenDates() = %v, want %v", gotWorkHours, tt.wantWorkHours)
			}
		})
	}
}
