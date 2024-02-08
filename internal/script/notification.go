package script

import (
	"errors"
)

var (
	ErrNotificationListIsEmpty         = errors.New("notification receiver list is empty")
	ErrOneOfSeveralStringFieldsIsEmpty = errors.New("one of several string fields is empty")
	ErrUnknownTextSourceType           = errors.New("unknown text source type")
	ErrEmptyText                       = errors.New("empty text text")
	ErrEmptyTextSourceRefValue         = errors.New("empty text source ref value")
)

type NotificationParams struct {
	People          []string   `json:"people"`
	Emails          []string   `json:"emails"`
	UsersFromSchema string     `json:"usersFromSchema"`
	Subject         string     `json:"subject"`
	Text            string     `json:"text"`
	TextSource      TextSource `json:"textSource"`
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

func (a *NotificationParams) validateStringFields() error {
	if a.Subject == "" {
		return ErrOneOfSeveralStringFieldsIsEmpty
	}

	err := a.validateText()
	if err != nil {
		return err
	}

	return nil
}

func (a *NotificationParams) validateText() error {
	switch a.TextSource.Type() {
	case OwnValueSource:
		return a.validateOwnValueTextSourceType()
	case ContextValueSource:
		return a.validateContextValueSourceType()
	default:
		return ErrUnknownTextSourceType
	}
}

func (a *NotificationParams) validateOwnValueTextSourceType() error {
	if a.TextSource.Text != "" {
		return nil
	}

	if a.Text == "" {
		return ErrEmptyText
	}

	return nil
}

func (a *NotificationParams) validateContextValueSourceType() error {
	if a.TextSource.RefValue == "" {
		return ErrEmptyTextSourceRefValue
	}

	return nil
}

type TextSourceType string

const (
	OwnValueSource     TextSourceType = "own_value"
	ContextValueSource TextSourceType = "context_value"
)

type TextSource struct {
	SourceType TextSourceType `json:"sourceType"`
	Text       string         `json:"text"`
	RefValue   string         `json:"refValue"`
}

func (ts *TextSource) Type() TextSourceType {
	if ts.SourceType == "" {
		return OwnValueSource
	}

	return ts.SourceType
}
