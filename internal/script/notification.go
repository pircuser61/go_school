package script

import (
	"errors"
)

type NotificationParams struct {
	People  []string `json:"people"`
	Subject string   `json:"subject"`
	Text    string   `json:"text"`
}

func (a *NotificationParams) Validate() error {
	if len(a.People) == 0 {
		return errors.New("notification people are empty")
	}

	if a.Subject == "" || a.Text == "" {
		return errors.New("got no text or subject")
	}

	return nil
}
