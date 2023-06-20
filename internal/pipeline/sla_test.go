package pipeline

import (
	"testing"
	"time"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/hrgate"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
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
			if check := CheckBreachSLA(tt.fields.Start, tt.fields.Current, tt.fields.SLA, nil); check != tt.wantedCheck {
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
			if deadline := ComputeDeadline(tt.fields.Start, tt.fields.SLA, nil); deadline != tt.wanted {
				t.Errorf("compute deadline returned unexpected result")
			}
		})
	}
}

func Test_getWorkWorkHoursBetweenDates(t *testing.T) {
	type fields struct {
		from       time.Time
		to         time.Time
		slaInfoPtr *SLAInfo
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
			wantWorkHours: 1,
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
			name: "work days eq 8",
			fields: fields{
				from: time.Date(2022, 07, 16, 0, 0, 0, 0, time.UTC),
				to:   time.Date(2022, 07, 18, 15, 30, 0, 0, time.UTC),
			},
			wantWorkHours: 8,
		},
		{
			name: "work days eq 7 at preholidays days",
			fields: fields{
				from: time.Date(2022, 07, 16, 0, 0, 0, 0, time.UTC),
				to:   time.Date(2022, 07, 18, 15, 30, 0, 0, time.UTC),
				slaInfoPtr: &SLAInfo{
					CalendarDays: &hrgate.CalendarDays{
						CalendarMap: map[int64]hrgate.CalendarDayType{time.Date(2022, 07, 16, 0, 0, 0, 0, time.UTC).Unix(): hrgate.CalendarDayTypePreHoliday, time.Date(2022, 07, 17, 0, 0, 0, 0, time.UTC).Unix(): hrgate.CalendarDayTypePreHoliday, time.Date(2022, 07, 18, 0, 0, 0, 0, time.UTC).Unix(): hrgate.CalendarDayTypePreHoliday},
					},
				},
			},
			wantWorkHours: 7,
		},
		{
			name: "work hours eq 0 at holidays days",
			fields: fields{
				from: time.Date(2022, 07, 18, 0, 0, 0, 0, time.UTC),
				to:   time.Date(2022, 07, 21, 15, 30, 0, 0, time.UTC),
				slaInfoPtr: &SLAInfo{
					CalendarDays: &hrgate.CalendarDays{
						CalendarMap: map[int64]hrgate.CalendarDayType{time.Date(2022, 07, 18, 0, 0, 0, 0, time.UTC).Unix(): hrgate.CalendarDayTypeHoliday, time.Date(2022, 07, 19, 0, 0, 0, 0, time.UTC).Unix(): hrgate.CalendarDayTypeHoliday, time.Date(2022, 07, 20, 0, 0, 0, 0, time.UTC).Unix(): hrgate.CalendarDayTypeHoliday, time.Date(2022, 07, 21, 0, 0, 0, 0, time.UTC).Unix(): hrgate.CalendarDayTypeHoliday},
					},
				},
			},
			wantWorkHours: 0,
		},
		{
			name: "work days with preholidays and holidays days",
			fields: fields{
				from: time.Date(2022, 07, 19, 0, 0, 0, 0, time.UTC),
				to:   time.Date(2022, 07, 21, 15, 30, 0, 0, time.UTC),
				slaInfoPtr: &SLAInfo{
					CalendarDays: &hrgate.CalendarDays{
						CalendarMap: map[int64]hrgate.CalendarDayType{time.Date(2022, 07, 20, 0, 0, 0, 0, time.UTC).Unix(): hrgate.CalendarDayTypeHoliday, time.Date(2022, 07, 21, 0, 0, 0, 0, time.UTC).Unix(): hrgate.CalendarDayTypeHoliday, time.Date(2022, 07, 18, 0, 0, 0, 0, time.UTC).Unix(): hrgate.CalendarDayTypeHoliday, time.Date(2022, 07, 19, 0, 0, 0, 0, time.UTC).Unix(): hrgate.CalendarDayTypePreHoliday},
					},
				},
			},
			wantWorkHours: 7,
		},
		{
			name: "work days with work type 24/7 without holidays",
			fields: fields{
				from: time.Date(2022, 07, 1, 0, 0, 0, 0, time.UTC),
				to:   time.Date(2022, 07, 15, 0, 0, 0, 0, time.UTC),
				slaInfoPtr: &SLAInfo{
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
				from: time.Date(2022, 07, 1, 0, 0, 0, 0, time.UTC),
				to:   time.Date(2022, 07, 15, 0, 0, 0, 0, time.UTC),
				slaInfoPtr: &SLAInfo{
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
				from: time.Date(2022, 07, 1, 0, 0, 0, 0, time.UTC),
				to:   time.Date(2022, 07, 15, 0, 0, 0, 0, time.UTC),
				slaInfoPtr: &SLAInfo{
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
				from: time.Date(2022, 07, 1, 0, 0, 0, 0, time.UTC),
				to:   time.Date(2022, 07, 15, 0, 0, 0, 0, time.UTC),
				slaInfoPtr: &SLAInfo{
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
				from: time.Date(2022, 07, 1, 0, 0, 0, 0, time.UTC),
				to:   time.Date(2022, 07, 15, 0, 0, 0, 0, time.UTC),
				slaInfoPtr: &SLAInfo{
					StartWorkHourPtr: utils.GetAddressOfValue(6),
					EndWorkHourPtr:   utils.GetAddressOfValue(14),
					Weekends:         []time.Weekday{time.Monday},
					CalendarDays:     &hrgate.CalendarDays{CalendarMap: map[int64]hrgate.CalendarDayType{time.Date(2022, 07, 13, 0, 0, 0, 0, time.UTC).Unix(): hrgate.CalendarDayTypeHoliday, time.Date(2022, 07, 12, 0, 0, 0, 0, time.UTC).Unix(): hrgate.CalendarDayTypePreHoliday}},
				},
			},
			wantWorkHours: 87,
		},
		{
			name: "work days with work type 24/7 without holidays",
			fields: fields{
				from: time.Date(2022, 07, 1, 0, 0, 0, 0, time.UTC),
				to:   time.Date(2022, 07, 15, 0, 0, 0, 0, time.UTC),
				slaInfoPtr: &SLAInfo{
					StartWorkHourPtr: utils.GetAddressOfValue(6),
					EndWorkHourPtr:   utils.GetAddressOfValue(14),
					Weekends:         []time.Weekday{time.Tuesday},
					CalendarDays:     &hrgate.CalendarDays{CalendarMap: map[int64]hrgate.CalendarDayType{time.Date(2022, 07, 13, 0, 0, 0, 0, time.UTC).Unix(): hrgate.CalendarDayTypeHoliday, time.Date(2022, 07, 14, 0, 0, 0, 0, time.UTC).Unix(): hrgate.CalendarDayTypeHoliday}},
				},
			},
			wantWorkHours: 80,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotWorkHours := getWorkHoursBetweenDates(tt.fields.from, tt.fields.to, tt.fields.slaInfoPtr); gotWorkHours != tt.wantWorkHours {
				t.Errorf("getWorkHoursBetweenDates() = %v, want %v", gotWorkHours, tt.wantWorkHours)
			}
		})
	}
}

func Test_ComputeMaxDate(t *testing.T) {
	type fields struct {
		from         time.Time
		workHourType *WorkHourType
		days         int
	}
	tests := []struct {
		name          string
		fields        fields
		wantTimestamp int64
	}{
		{
			name: "default test 8/5",
			fields: fields{
				from:         time.Date(2023, 6, 14, 6, 0, 0, 0, time.UTC),
				workHourType: utils.GetAddressOfValue(WorkTypeN85),
				days:         2,
			},
			wantTimestamp: time.Date(2023, 6, 15, 14, 0, 0, 0, time.UTC).Unix(),
		},
		{
			name: "default test 12/5",
			fields: fields{
				from:         time.Date(2023, 6, 5, 6, 0, 0, 0, time.UTC),
				workHourType: utils.GetAddressOfValue(WorkTypeN125),
				days:         2,
			},
			wantTimestamp: time.Date(2023, 6, 6, 18, 0, 0, 0, time.UTC).Unix(),
		},
		{
			name: "default test 24/7",
			fields: fields{
				from:         time.Date(2023, 5, 5, 6, 0, 0, 0, time.UTC),
				workHourType: utils.GetAddressOfValue(WorkTypeN247),
				days:         2,
			},
			wantTimestamp: time.Date(2023, 5, 7, 6, 0, 0, 0, time.UTC).Unix(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			startHour, endHour, _ := tt.fields.workHourType.GetWorkingHours()
			weekends, _ := tt.fields.workHourType.GetWeekends()
			totalSLA, _ := tt.fields.workHourType.GetTotalSLAInHours(tt.fields.days)
			if gotDate := ComputeMaxDate(tt.fields.from, float32(totalSLA), &SLAInfo{
				StartWorkHourPtr: &startHour,
				EndWorkHourPtr:   &endHour,
				Weekends:         weekends,
			}); gotDate.Unix() != tt.wantTimestamp {
				t.Errorf("ComputeMaxDate() = %v, want %v", gotDate.Format(time.RFC3339), time.Unix(tt.wantTimestamp, 0).Format(time.RFC3339))
			}
		})
	}
}
