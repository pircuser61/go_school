package pipeline

import (
	"testing"
	"time"
)

func Test_CheckBreachSLA(t *testing.T) {
	type fields struct {
		Start time.Time
		End   time.Time
		SLA   int
	}
	tests := []struct {
		name        string
		fields      fields
		wantedCheck bool
	}{
		{
			name: "bad sla",
			fields: fields{
				Start: time.Date(2022, 01, 03, 6, 0, 0, 0, time.UTC),
				End:   time.Date(2022, 01, 03, 5, 0, 0, 0, time.UTC),
				SLA:   1,
			},
			wantedCheck: true,
		},
		{
			name: "ok sla",
			fields: fields{
				Start: time.Date(2022, 01, 03, 6, 0, 0, 0, time.UTC),
				End:   time.Date(2022, 01, 03, 6, 30, 0, 0, time.UTC),
				SLA:   1,
			},
			wantedCheck: false,
		},
		{
			name: "ok sla next day",
			fields: fields{
				Start: time.Date(2022, 01, 03, 14, 0, 0, 0, time.UTC),
				End:   time.Date(2022, 01, 04, 6, 30, 0, 0, time.UTC),
				SLA:   2,
			},
			wantedCheck: false,
		},
		{
			name: "ok sla (now before worktime)",
			fields: fields{
				Start: time.Date(2022, 01, 03, 5, 0, 0, 0, time.UTC),
				End:   time.Date(2022, 01, 03, 6, 30, 0, 0, time.UTC),
				SLA:   1,
			},
			wantedCheck: false,
		},
		{
			name: "ok sla next day (now after worktime)",
			fields: fields{
				Start: time.Date(2022, 01, 03, 18, 0, 0, 0, time.UTC),
				End:   time.Date(2022, 01, 04, 6, 30, 0, 0, time.UTC),
				SLA:   1,
			},
			wantedCheck: false,
		},
		{
			name: "ok sla after weekend month",
			fields: fields{
				Start: time.Date(2022, 01, 07, 18, 0, 0, 0, time.UTC),
				End:   time.Date(2022, 02, 10, 6, 30, 0, 0, time.UTC),
				SLA:   1,
			},
			wantedCheck: false,
		},
		{
			name: "ok sla next month",
			fields: fields{
				Start: time.Date(2022, 01, 31, 18, 0, 0, 0, time.UTC),
				End:   time.Date(2022, 02, 01, 6, 30, 0, 0, time.UTC),
				SLA:   1,
			},
			wantedCheck: false,
		},
		{
			name: "ok sla next year",
			fields: fields{
				Start: time.Date(2022, 12, 31, 18, 0, 0, 0, time.UTC),
				End:   time.Date(2023, 01, 02, 6, 30, 0, 0, time.UTC),
				SLA:   1,
			},
			wantedCheck: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if check := CheckBreachSLA(tt.fields.Start, tt.fields.End, tt.fields.SLA); check != tt.wantedCheck {
				t.Errorf("check SLA returned unexpected result")
			}
		})
	}
}
