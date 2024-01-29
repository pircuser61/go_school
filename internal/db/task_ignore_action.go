package db

import (
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"golang.org/x/exp/slices"
)

func (db *PGCon) ignoreAction(a *entity.TaskAction, actionsToIgnore []IgnoreActionRule, computedActionIds []string) bool {
	for _, actionRule := range actionsToIgnore {
		if a.ID == actionRule.IgnoreActionID && slices.Contains(computedActionIds, actionRule.ExistingActionID) {
			return true
		}
	}

	return false
}
