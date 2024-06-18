package stephandlers

import (
	"encoding/json"

	"golang.org/x/exp/slices"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

const hiddenUserLogin = "hidden_user"

type HideExecutorsFormBlockStepHandler struct {
	stepDelegates  map[string]bool
	members        []string
	requesterLogin string
	isInitiator    bool
}

func NewHideExecutorsFormBlockStepHandler(
	stepDelegates map[string]bool,
	members []string,
	requesterLogin string,
) *HideExecutorsFormBlockStepHandler {
	return &HideExecutorsFormBlockStepHandler{
		stepDelegates:  stepDelegates,
		members:        members,
		requesterLogin: requesterLogin,
	}
}

func (h *HideExecutorsFormBlockStepHandler) HandleStep(step *entity.Step) error {
	if step.State == nil {
		return nil
	}

	if h.stepDelegates[step.Name] {
		return nil
	}

	var formBlock pipeline.FormData

	unmarshalErr := json.Unmarshal(step.State[step.Name], &formBlock)
	if unmarshalErr != nil {
		return unmarshalErr
	}

	userInMembers := slices.Contains(h.members, h.requesterLogin)
	userAsValidMember := !h.isInitiator || utils.IsContainsInMap(h.requesterLogin, formBlock.Executors)

	if !formBlock.HideExecutorFromInitiator || (userInMembers && userAsValidMember) {
		return nil
	}

	formBlock.Executors = map[string]struct{}{
		hiddenUserLogin: {},
	}

	formBlock.ActualExecutor = utils.GetAddressOfValue(hiddenUserLogin)

	for historyIdx := range formBlock.ChangesLog {
		formBlock.ChangesLog[historyIdx].Executor = hiddenUserLogin
		formBlock.ChangesLog[historyIdx].DelegateFor = hideDelegator(formBlock.ChangesLog[historyIdx].DelegateFor)
	}

	data, marshalErr := json.Marshal(formBlock)
	if marshalErr != nil {
		return marshalErr
	}

	step.State[step.Name] = data

	return nil
}

type HideExecutorsExecutionBlockStepHandler struct {
	stepDelegates  map[string]bool
	members        []string
	requesterLogin string
	isInitiator    bool
}

func NewHideExecutorsExecutionBlockStepHandler(
	stepDelegates map[string]bool,
	members []string,
	requesterLogin string,
) *HideExecutorsExecutionBlockStepHandler {
	return &HideExecutorsExecutionBlockStepHandler{
		stepDelegates:  stepDelegates,
		members:        members,
		requesterLogin: requesterLogin,
	}
}

func (h *HideExecutorsExecutionBlockStepHandler) HandleStep(step *entity.Step) error {
	if step.State == nil {
		return nil
	}

	if h.stepDelegates[step.Name] {
		return nil
	}

	var execBlock pipeline.ExecutionData

	unmarshalErr := json.Unmarshal(step.State[step.Name], &execBlock)
	if unmarshalErr != nil {
		return unmarshalErr
	}

	userInMembers := slices.Contains(h.members, h.requesterLogin)
	userAsValidMember := !h.isInitiator || utils.IsContainsInMap(h.requesterLogin, execBlock.Executors)

	if !execBlock.HideExecutor || (userInMembers && userAsValidMember) {
		return nil
	}

	execBlock.Executors = map[string]struct{}{
		hiddenUserLogin: {},
	}

	execBlock.InitialExecutors = map[string]struct{}{
		hiddenUserLogin: {},
	}

	if execBlock.ActualExecutor != nil {
		execBlock.ActualExecutor = utils.GetAddressOfValue(hiddenUserLogin)
	}

	for i := range execBlock.ChangedExecutorsLogs {
		execBlock.ChangedExecutorsLogs[i].OldLogin = hiddenUserLogin
		execBlock.ChangedExecutorsLogs[i].NewLogin = hiddenUserLogin
		execBlock.ChangedExecutorsLogs[i].Comment = ""
		execBlock.ChangedExecutorsLogs[i].ByLogin = hiddenUserLogin
		execBlock.ChangedExecutorsLogs[i].DelegateFor = hideDelegator(execBlock.ChangedExecutorsLogs[i].DelegateFor)
	}

	for i := range execBlock.TakenInWorkLog {
		execBlock.TakenInWorkLog[i].Executor = hiddenUserLogin
		execBlock.TakenInWorkLog[i].DelegateFor = hideDelegator(execBlock.TakenInWorkLog[i].DelegateFor)
	}

	for i := range execBlock.RequestExecutionInfoLogs {
		if execBlock.RequestExecutionInfoLogs[i].ReqType == pipeline.RequestInfoQuestion {
			execBlock.RequestExecutionInfoLogs[i].Login = hiddenUserLogin
			execBlock.RequestExecutionInfoLogs[i].DelegateFor = hideDelegator(execBlock.RequestExecutionInfoLogs[i].DelegateFor)
		}
	}

	for i := range execBlock.EditingAppLog {
		execBlock.EditingAppLog[i].Executor = hiddenUserLogin
		execBlock.EditingAppLog[i].DelegateFor = hideDelegator(execBlock.EditingAppLog[i].DelegateFor)
	}

	if execBlock.EditingApp != nil {
		execBlock.EditingApp.Executor = hiddenUserLogin
		execBlock.EditingApp.DelegateFor = hideDelegator(execBlock.EditingApp.DelegateFor)
	}

	data, marshalErr := json.Marshal(execBlock)
	if marshalErr != nil {
		return marshalErr
	}

	step.State[step.Name] = data

	return nil
}
