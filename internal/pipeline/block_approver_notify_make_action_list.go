package pipeline

import "gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"

func (gb *GoApproverBlock) makeActionList() []mail.Action {
	actionsList := make([]mail.Action, 0, len(gb.State.ActionList))

	for i := range gb.State.ActionList {
		actionsList = append(actionsList, mail.Action{
			InternalActionName: gb.State.ActionList[i].ID,
			Title:              gb.State.ActionList[i].Title,
		})
	}

	return actionsList
}
