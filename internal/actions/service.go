package actions

import (
	"encoding/json"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
)

type Service struct {
}

func New() {

}

type Action struct {
	Id        string
	BlockType string
	Type      string
	Title     string
}

type ActionData struct {
	Id        string
	BlockType string
	Type      string
	Title     string
}

const (
	ApproverBlockType  = "approver"
	FormBlockType      = "form"
	ExecutionBlockType = "execution"
)

type ApproverData struct {
	Approvers  map[string]struct{} `json:"approvers"`
	ActionList []ActionData        `json:"action_list"`
}

type FormData struct {
	Executors map[string]struct{} `json:"executors"`
}

type ExecutionData struct {
	Executors map[string]struct{} `json:"executors"`
}

func (as *Service) GetAvailableActionsFromTask(user *sso.UserInfo, task *entity.EriusTask) (actions []Action, err error) {
	var availableActions = make([]Action, 0)
	var actionsList = make([]Action, 0)

	// Инициатор:
	// - Отозвать
	// Формы:
	// - Заполнить форму
	// Согласование:
	// - Согласовать (primary)
	// - Отклонить (secondary)
	// - Добавить согласующего (extra)
	// - Вернуть на доработку
	// Исполнение
	// - Взять в работу (primary) (доступна если не взято в работу)
	// - ?

	var activeBlockIds = make([]string, 0)

	for activeBlockKey := range task.ActiveBlocks {
		activeBlockIds = append(activeBlockIds, activeBlockKey)
	}

	var activeBlocks = make([]*entity.Step, 0)

	for _, step := range task.Steps {
		for _, activeBlockId := range activeBlockIds {
			if step.Name == activeBlockId {
				activeBlocks = append(activeBlocks, step)
			}
		}
	}

	var currentUsername = user.Username
	var usernames = make([]string, 0)

	for _, activeBlock := range activeBlocks {
		switch activeBlock.Type {
		case FormBlockType:
			usernames, err = getFormExecutors(activeBlock)
			if err != nil {
				return []Action{}, err
			}
		case ApproverBlockType:
			usernames, err = getApprovers(activeBlock)
			if err != nil {
				return []Action{}, err
			}
		case ExecutionBlockType:
			usernames, err = getExecutors(activeBlock)
			if err != nil {
				return []Action{}, err
			}
		default:
			continue
		}

		// appending all determined actions for certain type
		if isStringEntryExist(currentUsername, usernames) {
			var typedActions = getActionsForBlockType(activeBlock.Type, actionsList)
			availableActions = append(availableActions, typedActions...)
		}
	}

	// appending cancel app action if user also an author
	if checkIfAuthor(user, task) {
		availableActions = append(availableActions, Action{
			// добавляем отмену (cancel_app)
		})
	}

	return availableActions, nil
}

func (as *Service) GetActionsList() (actions []Action, err error) {
	return nil, nil
}

func getActionsForBlockType(blockType string, actions []Action) []Action {
	var result = make([]Action, 0)

	for _, action := range actions {
		if action.Type == blockType {
			result = append(result, action)
		}
	}

	return result
}

func checkIfAuthor(user *sso.UserInfo, task *entity.EriusTask) bool {
	return task.Author == user.Username
}

func getApprovers(activeBlock *entity.Step) (approvers []string, err error) {
	approvers = make([]string, 0)
	blockData := ApproverData{}
	if unmarshalErr := json.Unmarshal(activeBlock.State[activeBlock.Name], &blockData); unmarshalErr != nil {
		return []string{}, unmarshalErr
	}

	for formExecutorKey := range blockData.Approvers {
		approvers = append(approvers, formExecutorKey)
	}

	return approvers, nil
}

func getExecutors(activeBlock *entity.Step) (executors []string, err error) {
	executors = make([]string, 0)
	blockData := ExecutionData{}
	if unmarshalErr := json.Unmarshal(activeBlock.State[activeBlock.Name], &blockData); unmarshalErr != nil {
		return []string{}, unmarshalErr
	}

	for formExecutorKey := range blockData.Executors {
		executors = append(executors, formExecutorKey)
	}

	return executors, nil
}

func getFormExecutors(activeBlock *entity.Step) (formExecutors []string, err error) {
	formExecutors = make([]string, 0)
	blockData := FormData{}
	if unmarshalErr := json.Unmarshal(activeBlock.State[activeBlock.Name], &blockData); unmarshalErr != nil {
		return []string{}, unmarshalErr
	}

	for formExecutorKey := range blockData.Executors {
		formExecutors = append(formExecutors, formExecutorKey)
	}

	return formExecutors, nil
}

func isStringEntryExist(entry string, source []string) bool {
	for _, a := range source {
		if a == entry {
			return true
		}
	}
	return false
}
