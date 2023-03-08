package hrgate

import "github.com/google/uuid"

type Employee struct {
	Id       uuid.UUID `json:"id"`
	Activity struct {
		From string `json:"from"`
		To   string `json:"to"`
	} `json:"activity"`
	CorpTaxiAgreement   bool      `json:"corpTaxiAgreement"`
	Email               string    `json:"email"`
	Login               string    `json:"login"`
	OrganizationId      uuid.UUID `json:"organizationId"`
	PdProcessingAllowed bool      `json:"pdProcessingAllowed"`
	PersonId            uuid.UUID `json:"personId"`
	Phone               string    `json:"phone"`
	TabNum              string    `json:"tabNum"`
	TypeId              uuid.UUID `json:"typeID"`
	Primary             bool      `json:"primary"`
}

type Employees []*Employee

type Organization struct {
	Id   uuid.UUID `json:"id"`
	Unit struct {
		Id         uuid.UUID `json:"id"`
		Title      string    `json:"title"`
		UnitTypeId uuid.UUID `json:"unitTypeId"`
	} `json:"unit"`
}

type Calendar struct {
	Id              uuid.UUID `json:"id"`
	HolidayCalendar string    `json:"holidayCalendar"`
	Primary         bool      `json:"primary"`
	UnitID          uuid.UUID `json:"unitID"`
	WeekType        string    `json:"weekType"`
}

type Calendars []*Calendar

type CalendarDay struct {
	Id         uuid.UUID `json:"id"`
	CalendarId uuid.UUID `json:"calendarId"`
	Date       string    `json:"date"`
	DayType    string    `json:"dayType"`
}

type CalendarDays []*CalendarDay
