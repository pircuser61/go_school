package script

import (
	"errors"
)

type NotificationParams struct {
	People          []string `json:"people"`
	Emails          []string `json:"emails"`
	UsersFromSchema string   `json:"usersFromSchema"`
	Subject         string   `json:"subject"`
	Text            string   `json:"text"`
}

func (a *NotificationParams) Validate() error {
	if len(a.People) == 0 && len(a.Emails) == 0 && a.UsersFromSchema == "" {
		return errors.New("notification receiver list is empty")
	}

	if a.Subject == "" || a.Text == "" {
		return errors.New("one of several string fields is empty")
	}

	return nil
}
