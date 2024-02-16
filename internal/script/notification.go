package script

import (
	"errors"
)

var (
	ErrNotificationListIsEmpty         = errors.New("notification receiver list is empty")
	ErrOneOfSeveralStringFieldsIsEmpty = errors.New("one of several string fields is empty")
	ErrUnknownTextSourceType           = errors.New("unknown text source type")
	ErrEmptyText                       = errors.New("empty text text")
)

type NotificationParams struct {
	People          []string       `json:"people"`
	Emails          []string       `json:"emails"`
	UsersFromSchema string         `json:"usersFromSchema"`
	Subject         string         `json:"subject"`
	Text            string         `json:"text"`
	TextSourceType  TextSourceType `json:"textSourceType"`
}

func (a *NotificationParams) Validate() error {
	if len(a.People) == 0 && len(a.Emails) == 0 && a.UsersFromSchema == "" {
		return ErrNotificationListIsEmpty
	}

	err := a.validateStringFields()
	if err != nil {
		return err
	}

	return nil
}

func (a *NotificationParams) Type() TextSourceType {
	if a.TextSourceType == "" {
		return TextFieldSource
	}

	return a.TextSourceType
}

func (a *NotificationParams) validateStringFields() error {
	if a.Subject == "" {
		return ErrOneOfSeveralStringFieldsIsEmpty
	}

	err := a.validateTextSource()
	if err != nil {
		return err
	}

	return nil
}

//nolint:exhaustive //нам не нужны остальные случаи
func (a *NotificationParams) validateTextSource() error {
	switch a.Type() {
	case TextFieldSource:
		return a.validateText()
	default:
		return ErrUnknownTextSourceType
	}
}

func (a *NotificationParams) validateText() error {
	if a.Text == "" {
		return ErrEmptyText
	}

	return nil
}

type TextSourceType string

const (
	TextFieldSource  TextSourceType = "field"
	VarContextSource TextSourceType = "context"
)
