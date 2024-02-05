package stephandlers

import (
	"golang.org/x/exp/slices"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/humantasks"
)

func isDelegate(currentUser, login string, delegations *humantasks.Delegations) bool {
	delegates := delegations.GetDelegates(login)

	return slices.Contains(delegates, currentUser)
}

func hideDelegator(delegate string) string {
	if delegate == "" {
		return ""
	}

	return hiddenUserLogin
}
