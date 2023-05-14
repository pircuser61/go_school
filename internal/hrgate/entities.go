package hrgate

type CalendarDays struct {
	Holidays    []int64 `json:"holidays"`
	PreHolidays []int64 `json:"pre_holidays"`
	WorkDay     []int64 `json:"work_day"`
	Weekend     []int64 `json:"weekend"`
}
