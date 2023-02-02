package utils

import "strings"

func GetLoginFromEmail(email string) (login string) {
	emailParts := strings.Split(email, "@")
	if len(emailParts) == 2 {
		login = emailParts[0]
	} else {
		login = email
	}
	return
}
