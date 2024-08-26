package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sla"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

const (
	keyOutputExecutionLogin    = "login"
	keyOutputExecutionDecision = "decision"
	keyOutputExecutionComment  = "comment"

	ExecutionDecisionExecuted ExecutionDecision = "executed"
	ExecutionDecisionRejected ExecutionDecision = "rejected"
	ExecutionDecisionSentEdit ExecutionDecision = "sent_edit"

	RequestInfoQuestion RequestInfoType = "question"
	RequestInfoAnswer   RequestInfoType = "answer"

	executionStartWorkAction            = "executor_start_work"
	executionSendEditAppAction          = "executor_send_edit_app"
	executionChangeExecutorAction       = "change_executor"
	executionRequestExecutionInfoAction = "request_execution_info"
	executionExecuteAction              = "execution"
	executionDeclineAction              = "decline"
	executionBackToGroup                = "back_to_group"
	executionNewExecutionTask           = "new_execution_task"
)

type GoExecutionBlock struct {
	Name      string
	ShortName string
	Title     string
	Input     map[string]string
	Output    map[string]string
	Sockets   []script.Socket
	State     *ExecutionData

	RunContext *BlockRunContext

	expectedEvents      map[string]struct{}
	happenedEvents      []entity.NodeEvent
	happenedKafkaEvents []entity.NodeKafkaEvent
}

func mapToSlice(data map[string]struct{}) []string {
	keys := make([]string, 0, len(data))

	for k := range data {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	return keys
}

func (gb *GoExecutionBlock) CurrentExecutorData() CurrentExecutorData {
	return CurrentExecutorData{
		GroupID:       gb.State.ExecutorsGroupID,
		GroupName:     gb.State.ExecutorsGroupName,
		People:        mapToSlice(gb.State.Executors),
		InitialPeople: mapToSlice(gb.State.InitialExecutors),
		GroupLimit:    gb.State.ExecutorsGroupLimit,
	}
}

func (gb *GoExecutionBlock) GetNewEvents() []entity.NodeEvent {
	return gb.happenedEvents
}

func (gb *GoExecutionBlock) GetNewKafkaEvents() []entity.NodeKafkaEvent {
	return gb.happenedKafkaEvents
}

func (gb *GoExecutionBlock) getDeadline(ctx context.Context, workType string) (time.Time, error) {
	slaInfoPtr, getSLAInfoErr := gb.RunContext.Services.SLAService.GetSLAInfoPtr(ctx, sla.InfoDTO{
		TaskCompletionIntervals: []entity.TaskCompletionInterval{{
			StartedAt:  gb.RunContext.CurrBlockStartTime,
			FinishedAt: gb.RunContext.CurrBlockStartTime.Add(time.Hour * 24 * 100),
		}},
		WorkType: sla.WorkHourType(workType),
	})
	if getSLAInfoErr != nil {
		return time.Time{}, errors.Wrap(getSLAInfoErr, "can not get slaInfo")
	}

	return gb.getNewSLADeadline(slaInfoPtr, false), nil
}

func (gb *GoExecutionBlock) Members() []Member {
	members := make([]Member, 0, len(gb.State.Executors))
	addedMembers := make(map[string]struct{}, len(gb.State.Executors))

	for login := range gb.State.Executors {
		members = append(members, Member{
			Login:                login,
			Actions:              gb.executionActions(login),
			IsActed:              gb.isExecutionActed(login),
			ExecutionGroupMember: gb.isPartOfExecutionGroup(login),
		})

		addedMembers[login] = struct{}{}
	}

	for i := 0; i < len(gb.State.EditingAppLog); i++ {
		log := gb.State.EditingAppLog[i]
		if _, ok := addedMembers[log.Executor]; !ok {
			continue
		}

		members = append(members, Member{
			Login:                log.Executor,
			Actions:              []MemberAction{},
			IsActed:              true,
			ExecutionGroupMember: gb.isPartOfExecutionGroup(log.Executor),
		})

		addedMembers[log.Executor] = struct{}{}
	}

	for i := 0; i < len(gb.State.RequestExecutionInfoLogs); i++ {
		log := gb.State.RequestExecutionInfoLogs[i]
		if _, ok := addedMembers[log.Login]; !ok {
			continue
		}

		if log.ReqType == RequestInfoQuestion {
			members = append(members, Member{
				Login:                log.Login,
				Actions:              []MemberAction{},
				IsActed:              true,
				ExecutionGroupMember: gb.isPartOfExecutionGroup(log.Login),
			})

			addedMembers[log.Login] = struct{}{}
		}
	}

	latestInfoRequest := gb.State.latestUnansweredAddInfoLogEntry()
	isQuestionExist := latestInfoRequest != nil && latestInfoRequest.ReqType == RequestInfoQuestion

	if gb.State.EditingApp != nil {
		members = append(members, Member{
			Login: gb.RunContext.Initiator,
			Actions: []MemberAction{
				{
					ID:     string(entity.TaskUpdateActionEditApp),
					Type:   ActionTypeCustom,
					Params: map[string]interface{}{},
				},
			},
			IsActed:              false,
			ExecutionGroupMember: false,
			IsInitiator:          true,
		})
	}

	if isQuestionExist {
		members = append(members, Member{
			Login: gb.RunContext.Initiator,
			Actions: []MemberAction{
				{
					ID:     string(entity.TaskUpdateActionReplyExecutionInfo),
					Type:   ActionTypeCustom,
					Params: map[string]interface{}{},
				},
			},
			IsActed:              false,
			ExecutionGroupMember: false,
			IsInitiator:          true,
		})
	}

	for i := range gb.State.ChangedExecutorsLogs {
		if _, ok := addedMembers[gb.State.ChangedExecutorsLogs[i].OldLogin]; !ok {
			members = append(members, Member{
				Login:                gb.State.ChangedExecutorsLogs[i].OldLogin,
				Actions:              []MemberAction{},
				Finished:             true,
				IsActed:              true,
				ExecutionGroupMember: gb.isPartOfExecutionGroup(gb.State.ChangedExecutorsLogs[i].OldLogin),
			})
			addedMembers[gb.State.ChangedExecutorsLogs[i].OldLogin] = struct{}{}

			continue
		}

		members = append(members, Member{
			Login:                gb.State.ChangedExecutorsLogs[i].OldLogin,
			Actions:              []MemberAction{},
			IsActed:              true,
			ExecutionGroupMember: gb.isPartOfExecutionGroup(gb.State.ChangedExecutorsLogs[i].OldLogin),
		})

		addedMembers[gb.State.ChangedExecutorsLogs[i].OldLogin] = struct{}{}
	}

	if gb.State.ActualExecutor != nil {
		if _, ok := addedMembers[*gb.State.ActualExecutor]; !ok {
			members = append(members, Member{
				Login:                *gb.State.ActualExecutor,
				Actions:              []MemberAction{},
				IsActed:              true,
				ExecutionGroupMember: gb.isPartOfExecutionGroup(*gb.State.ActualExecutor),
			})

			addedMembers[*gb.State.ActualExecutor] = struct{}{}
		}
	}

	for key := range gb.State.InitialExecutors {
		if _, ok := addedMembers[key]; ok {
			continue
		}

		members = append(members, Member{
			Login:                key,
			Actions:              []MemberAction{},
			IsActed:              !gb.isPartOfExecutionGroup(key),
			ExecutionGroupMember: gb.isPartOfExecutionGroup(key),
		})
		addedMembers[key] = struct{}{}
	}

	return members
}

func (gb *GoExecutionBlock) isExecutionActed(login string) bool {
	if (gb.State.ActualExecutor != nil && *gb.State.ActualExecutor == login) && gb.State.Decision != nil {
		return true
	}

	for i := 0; i < len(gb.State.EditingAppLog); i++ {
		log := gb.State.EditingAppLog[i]
		if log.Executor == login || log.DelegateFor == login {
			return true
		}
	}

	for i := 0; i < len(gb.State.ChangedExecutorsLogs); i++ {
		log := gb.State.ChangedExecutorsLogs[i]
		if log.OldLogin == login {
			return true
		}
	}

	for i := 0; i < len(gb.State.RequestExecutionInfoLogs); i++ {
		log := gb.State.RequestExecutionInfoLogs[i]
		if (log.Login == login || log.DelegateFor == login) && log.ReqType == RequestInfoQuestion {
			return true
		}
	}

	return gb.State.IsTakenInWork
}

func (gb *GoExecutionBlock) isPartOfExecutionGroup(login string) bool {
	// nolint:exhaustive //не хотим обрабатывать остальные случаи
	switch gb.State.ExecutionType {
	case script.ExecutionTypeGroup:
		if _, ok := gb.State.InitialExecutors[login]; ok {
			return true
		}
	case script.ExecutionTypeFromSchema:
		if len(gb.State.InitialExecutors) > 1 {
			if _, ok := gb.State.InitialExecutors[login]; ok {
				return true
			}
		}
	default:
		return false
	}

	return false
}

func (gb *GoExecutionBlock) executionActions(login string) []MemberAction {
	if gb.State.Decision != nil || gb.State.EditingApp != nil {
		return nil
	}

	if !gb.State.IsTakenInWork {
		action := MemberAction{
			ID:   executionStartWorkAction,
			Type: ActionTypePrimary,
		}

		return []MemberAction{action}
	}

	actions := []MemberAction{
		{
			ID:   executionExecuteAction,
			Type: ActionTypePrimary,
		},
		{
			ID:   executionDeclineAction,
			Type: ActionTypeSecondary,
		},
		{
			ID:   executionChangeExecutorAction,
			Type: ActionTypeOther,
		},
		{
			ID:   executionRequestExecutionInfoAction,
			Type: ActionTypeOther,
		},
	}

	isDelegated := false

	l := len(gb.State.TakenInWorkLog)
	if l > 0 {
		delegate := gb.State.TakenInWorkLog[l-1].DelegateFor
		_, isDelegated = gb.State.InitialExecutors[delegate]
	}

	if _, ok := gb.State.InitialExecutors[login]; ok && gb.State.ExecutorsGroupID != "" || isDelegated {
		actions = append(actions, MemberAction{
			ID:   executionBackToGroup,
			Type: ActionTypeOther,
		})
	}

	if gb.State.IsEditable {
		actions = append(actions, MemberAction{
			ID:   executionSendEditAppAction,
			Type: ActionTypeOther,
		})
	}

	if gb.State.ChildWorkBlueprintID != nil && *gb.State.ChildWorkBlueprintID != "" {
		actions = append(actions, MemberAction{
			ID:   executionNewExecutionTask,
			Type: ActionTypeOther,
			Params: map[string]interface{}{
				"child_work_blueprint_id": *gb.State.ChildWorkBlueprintID,
			},
		})
	}

	fillFormNames, existEmptyForm := gb.getFormNamesToFill()
	if existEmptyForm {
		for i := 0; i < len(actions); i++ {
			item := &actions[i]

			if item.ID != executionExecuteAction {
				continue
			}

			item.Params = map[string]interface{}{
				"disabled":  true,
				description: fillFormMessage,
			}
		}
	}

	if len(fillFormNames) != 0 {
		actions = append(actions, MemberAction{
			ID:   formFillFormAction,
			Type: ActionTypeCustom,
			Params: map[string]interface{}{
				formName: fillFormNames,
			},
		})
	}

	return actions
}

func (gb *GoExecutionBlock) getFormNamesToFill() ([]string, bool) {
	var (
		actions   = make([]string, 0)
		emptyForm = false
		l         = logger.GetLogger(context.Background())
	)

	for _, form := range gb.State.FormsAccessibility {
		formState, ok := gb.RunContext.VarStore.State[form.NodeID]
		if !ok {
			continue
		}

		switch form.AccessType {
		case readWriteAccessType:
			actions = append(actions, form.NodeID)
		case requiredFillAccessType:
			actions = append(actions, form.NodeID)

			existEmptyForm := gb.checkForEmptyForm(formState, l)
			if existEmptyForm {
				emptyForm = true
			}
		}
	}

	return actions, emptyForm
}

func (gb *GoExecutionBlock) checkForEmptyForm(formState json.RawMessage, l logger.Logger) bool {
	var formData FormData
	if err := json.Unmarshal(formState, &formData); err != nil {
		l.Error(err)

		return true
	}

	users := make(map[string]struct{}, 0)

	for user := range gb.State.Executors {
		users[user] = struct{}{}
	}

	for user := range gb.State.InitialExecutors {
		users[user] = struct{}{}
	}

	for i := 0; i < len(gb.State.ChangedExecutorsLogs); i++ {
		item := gb.State.ChangedExecutorsLogs[i]
		users[item.OldLogin] = struct{}{}
	}

	if !formData.IsFilled {
		return true
	}

	for _, v := range formData.ChangesLog {
		if _, findOk := users[v.Executor]; findOk {
			return false
		}
	}

	return true
}

func (gb *GoExecutionBlock) getNewSLADeadline(slaInfoPtr *sla.Info, half bool) time.Time {
	newSLA := gb.State.SLA
	if half {
		newSLA /= 2
	}

	deadline := gb.RunContext.Services.SLAService.ComputeMaxDate(gb.RunContext.CurrBlockStartTime, float32(newSLA), slaInfoPtr)

	var qTime time.Time

	for _, item := range gb.State.RequestExecutionInfoLogs {
		if item.CreatedAt.Before(gb.RunContext.CurrBlockStartTime) {
			continue
		}

		if qTime.IsZero() {
			qTime = item.CreatedAt

			continue
		}
	}

	for _, item := range gb.State.RequestExecutionInfoLogs {
		if qTime.IsZero() {
			qTime = item.CreatedAt

			continue
		}

		additionalHours := gb.RunContext.Services.SLAService.GetWorkHoursBetweenDates(qTime, item.CreatedAt, nil)
		deadline = gb.RunContext.Services.SLAService.ComputeMaxDate(deadline, float32(additionalHours), nil)
		qTime = time.Time{}
	}

	return deadline
}

//nolint:dupl //Need here
func (gb *GoExecutionBlock) Deadlines(ctx context.Context) ([]Deadline, error) {
	deadlines := make([]Deadline, 0, 2)

	latestInfoRequest := gb.State.latestUnansweredAddInfoLogEntry()

	if gb.State.Decision != nil && latestInfoRequest != nil && latestInfoRequest.ReqType == RequestInfoQuestion {
		if gb.State.CheckDayBeforeSLARequestInfo {
			deadlines = append(deadlines, Deadline{
				Deadline: gb.RunContext.Services.SLAService.ComputeMaxDate(latestInfoRequest.CreatedAt,
					2*workingHours, nil),
				Action: entity.TaskUpdateActionDayBeforeSLARequestAddInfo,
			})
		}

		deadlines = append(deadlines, Deadline{
			Deadline: gb.RunContext.Services.SLAService.ComputeMaxDate(latestInfoRequest.CreatedAt,
				3*workingHours, nil),
			Action: entity.TaskUpdateActionSLABreachRequestAddInfo,
		})

		return deadlines, nil
	}

	if gb.State.Decision != nil {
		return []Deadline{}, nil
	}

	if gb.State.CheckSLA && (latestInfoRequest == nil || latestInfoRequest.ReqType == RequestInfoAnswer) {
		slaInfoPtr, getSLAInfoErr := gb.RunContext.Services.SLAService.GetSLAInfoPtr(ctx, sla.InfoDTO{
			TaskCompletionIntervals: []entity.TaskCompletionInterval{{
				StartedAt:  gb.RunContext.CurrBlockStartTime,
				FinishedAt: gb.RunContext.CurrBlockStartTime.Add(time.Hour * 24 * 100),
			}},
			WorkType: sla.WorkHourType(gb.State.WorkType),
		})

		if getSLAInfoErr != nil {
			return nil, getSLAInfoErr
		}

		gb.State.Deadline = gb.getNewSLADeadline(slaInfoPtr, false)

		if !gb.State.SLAChecked {
			deadlines = append(deadlines,
				Deadline{
					Deadline: gb.getNewSLADeadline(slaInfoPtr, false),
					Action:   entity.TaskUpdateActionSLABreach,
				},
			)
		}

		if !gb.State.HalfSLAChecked {
			deadlines = append(deadlines,
				Deadline{
					Deadline: gb.getNewSLADeadline(slaInfoPtr, true),
					Action:   entity.TaskUpdateActionHalfSLABreach,
				},
			)
		}
	}

	if gb.State.IsEditable && gb.State.CheckReworkSLA && gb.State.EditingApp != nil {
		deadlines = append(deadlines,
			Deadline{
				Deadline: gb.RunContext.Services.SLAService.ComputeMaxDate(
					gb.State.EditingApp.CreatedAt, float32(gb.State.ReworkSLA), nil),
				Action: entity.TaskUpdateActionReworkSLABreach,
			},
		)
	}

	if latestInfoRequest != nil && latestInfoRequest.ReqType == RequestInfoQuestion {
		if gb.State.CheckDayBeforeSLARequestInfo {
			deadlines = append(deadlines, Deadline{
				Deadline: gb.RunContext.Services.SLAService.ComputeMaxDate(latestInfoRequest.CreatedAt,
					2*workingHours, nil),
				Action: entity.TaskUpdateActionDayBeforeSLARequestAddInfo,
			})
		}

		deadlines = append(deadlines, Deadline{
			Deadline: gb.RunContext.Services.SLAService.ComputeMaxDate(latestInfoRequest.CreatedAt,
				3*workingHours, nil),
			Action: entity.TaskUpdateActionSLABreachRequestAddInfo,
		})
	}

	return deadlines, nil
}

func (gb *GoExecutionBlock) UpdateManual() bool {
	return true
}

// nolint:dupl // another block
func (gb *GoExecutionBlock) GetTaskHumanStatus() (status TaskHumanStatus, comment, action string) {
	if gb.State != nil && gb.State.Decision != nil {
		switch *gb.State.Decision {
		case ExecutionDecisionExecuted:
			return StatusDone, "", ""
		case ExecutionDecisionSentEdit:
			return StatusExecutionRejected, "", "отправлена на доработку"
		case ExecutionDecisionRejected:
			return StatusExecutionRejected, "", ""
		default:
			return "", "", ""
		}
	}

	if gb.State.EditingApp != nil {
		return StatusWait, "", ""
	}

	if len(gb.State.RequestExecutionInfoLogs) > 0 &&
		gb.State.RequestExecutionInfoLogs[len(gb.State.RequestExecutionInfoLogs)-1].ReqType == RequestInfoQuestion {
		return StatusWait, "", ""
	}

	return StatusExecution, "", ""
}

// nolint:dupl // another block
func (gb *GoExecutionBlock) GetStatus() Status {
	if gb.State != nil && gb.State.Decision != nil {
		if *gb.State.Decision == ExecutionDecisionRejected {
			return StatusNoSuccess
		}

		if *gb.State.Decision == ExecutionDecisionSentEdit {
			return StatusNoSuccess
		}

		return StatusFinished
	}

	if gb.State.EditingApp != nil {
		return StatusIdle
	}

	if len(gb.State.RequestExecutionInfoLogs) > 0 &&
		gb.State.RequestExecutionInfoLogs[len(gb.State.RequestExecutionInfoLogs)-1].ReqType == RequestInfoQuestion {
		return StatusIdle
	}

	return StatusRunning
}

func (gb *GoExecutionBlock) Next(_ *store.VariableStore) ([]string, bool) {
	key := notExecutedSocketID
	if gb.State != nil && gb.State.Decision != nil && *gb.State.Decision == ExecutionDecisionExecuted {
		key = executedSocketID
	}

	if gb.State != nil && gb.State.Decision != nil && *gb.State.Decision == ExecutionDecisionSentEdit {
		key = executionEditAppSocketID
	}

	// возврат заявки инициатору. эта заявка дальше не пойдет по процессу
	if gb.State != nil && gb.State.Decision == nil && gb.State.EditingApp != nil {
		return nil, false
	}

	nexts, ok := script.GetNexts(gb.Sockets, key)
	if !ok {
		return nil, false
	}

	return nexts, true
}

func (gb *GoExecutionBlock) GetState() interface{} {
	return gb.State
}

func (gb *GoExecutionBlock) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockGoExecutionID,
		BlockType: script.TypeGo,
		Title:     gb.Title,
		Inputs:    nil,
		Outputs: &script.JSONSchema{
			Type: "object",
			Properties: script.JSONSchemaProperties{
				keyOutputExecutionLogin: {
					Type:        "object",
					Description: "executor login",
					Format:      "SsoPerson",
					Properties:  people.GetSsoPersonSchemaProperties(),
				},
				keyOutputExecutionDecision: {
					Type:        "string",
					Description: "execution status",
				},
				keyOutputExecutionComment: {
					Type:        "string",
					Description: "execution status comment",
				},
			},
		},
		Params: &script.FunctionParams{
			Type: BlockGoExecutionID,
			Params: &script.ExecutionParams{
				Executors:          "",
				Type:               "",
				SLA:                0,
				FormsAccessibility: []script.FormAccessibility{},
			},
		},
		Sockets: []script.Socket{
			script.ExecutedSocket,
			script.NotExecutedSocket,
		},
	}
}

func (gb *GoExecutionBlock) BlockAttachments() (ids []string) {
	ids = make([]string, 0)

	for i := range gb.State.RequestExecutionInfoLogs {
		for j := range gb.State.RequestExecutionInfoLogs[i].Attachments {
			ids = append(ids, gb.State.RequestExecutionInfoLogs[i].Attachments[j].FileID)
		}
	}

	for i := range gb.State.DecisionAttachments {
		ids = append(ids, gb.State.DecisionAttachments[i].FileID)
	}

	for i := range gb.State.EditingAppLog {
		for j := range gb.State.EditingAppLog[i].Attachments {
			ids = append(ids, gb.State.EditingAppLog[i].Attachments[j].FileID)
		}
	}

	for i := range gb.State.ChangedExecutorsLogs {
		for j := range gb.State.ChangedExecutorsLogs[i].Attachments {
			ids = append(ids, gb.State.ChangedExecutorsLogs[i].Attachments[j].FileID)
		}
	}

	return utils.UniqueStrings(ids)
}

type ExecutionOutput struct {
	Login    *people.Person
	Comment  *string
	Decision *ExecutionDecision
}

func (gb *GoExecutionBlock) UpdateStateUsingOutput(_ context.Context, data []byte) (state map[string]interface{}, err error) {
	executionParams := ExecutionOutput{}

	unmErr := json.Unmarshal(data, &executionParams)
	if unmErr != nil {
		return nil, fmt.Errorf("can't unmarshal into output struct")
	}

	if executionParams.Decision != nil {
		gb.State.Decision = executionParams.Decision
	}

	if executionParams.Comment != nil {
		gb.State.DecisionComment = executionParams.Comment
	}

	if executionParams.Login != nil {
		gb.State.ActualExecutor = &executionParams.Login.Username
	}

	jsonState, marshErr := json.Marshal(gb.State)
	if marshErr != nil {
		return nil, marshErr
	}

	unmarshErr := json.Unmarshal(jsonState, &state)
	if unmarshErr != nil {
		return nil, unmarshErr
	}

	return state, nil
}

func (gb *GoExecutionBlock) UpdateOutputUsingState(ctx context.Context) (map[string]interface{}, error) {
	output := map[string]interface{}{}

	if gb.State.ActualExecutor != nil {
		ssoUser, ssoErr := gb.RunContext.Services.People.GetUser(ctx, *gb.State.ActualExecutor, false)
		if ssoErr != nil {
			return nil, ssoErr
		}

		person, errConv := ssoUser.ToPerson()
		if errConv != nil {
			return nil, errConv
		}

		output[keyOutputExecutionLogin] = person
	}

	if gb.State.Decision != nil {
		output[keyOutputDecision] = gb.State.Decision
	}

	if gb.State.DecisionComment != nil {
		output[keyOutputComment] = gb.State.DecisionComment
	}

	return output, nil
}
