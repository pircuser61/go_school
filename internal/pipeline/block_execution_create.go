package pipeline

import (
	c "context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

// nolint:dupl // another block
func createGoExecutionBlock(ctx c.Context, name string, ef *entity.EriusFunc, runCtx *BlockRunContext) (*GoExecutionBlock, bool, error) {
	log := logger.GetLogger(ctx)

	b := &GoExecutionBlock{
		Name:    name,
		Title:   ef.Title,
		Input:   map[string]string{},
		Output:  map[string]string{},
		Sockets: entity.ConvertSocket(ef.Sockets),

		RunContext: runCtx,
	}

	for _, v := range ef.Input {
		b.Input[v.Name] = v.Global
	}

	for _, v := range ef.Output {
		b.Output[v.Name] = v.Global
	}

	rawState, blockExists := runCtx.VarStore.State[name]
	reEntry := false
	if blockExists {
		if err := b.loadState(rawState); err != nil {
			return nil, false, err
		}

		reEntry = runCtx.UpdateData == nil
		log.WithField("createGoExecutionBlock", runCtx.UpdateData).
			Info("create new execution block: "+b.Name, reEntry)

		// это для возврата в рамках одного процесса
		if reEntry {
			if err := b.reEntry(ctx, ef); err != nil {
				return nil, false, err
			}
			b.RunContext.VarStore.AddStep(b.Name)
		}
	} else {
		if err := b.createState(ctx, ef); err != nil {
			return nil, false, err
		}
		b.RunContext.VarStore.AddStep(b.Name)

		// TODO: выпилить когда сделаем циклы
		// это для возврата на доработку при которой мы создаем новый процесс
		// и пытаемся взять решение из прошлого процесса
		if err := b.setPrevDecision(ctx); err != nil {
			return nil, false, err
		}
	}

	return b, reEntry, nil
}

func (gb *GoExecutionBlock) reEntry(ctx c.Context, ef *entity.EriusFunc) error {
	if gb.State.GetRepeatPrevDecision() {
		return nil
	}

	gb.State.Decision = nil
	gb.State.DecisionComment = nil
	gb.State.DecisionAttachments = make([]string, 0)
	gb.State.ActualExecutor = nil
	gb.State.IsTakenInWork = false

	var params script.ExecutionParams
	err := json.Unmarshal(ef.Params, &params)
	if err != nil {
		return errors.Wrap(err, "can not get execution parameters for block: "+gb.Name)
	}

	if params.ExecutorsGroupIDPath != nil {
		variableStorage, grabStorageErr := gb.RunContext.VarStore.GrabStorage()
		if grabStorageErr != nil {
			return grabStorageErr
		}

		groupId := getVariable(variableStorage, *params.ExecutorsGroupIDPath)
		if groupId == nil {
			return errors.New("can't find group id in variables")
		}
		params.ExecutorsGroupID = fmt.Sprintf("%v", groupId)
	}
	executorChosenFlag := false
	if gb.State.UseActualExecutor {
		execs, prevErr := gb.RunContext.Storage.GetExecutorsFromPrevExecutionBlockRun(ctx, gb.RunContext.TaskID, gb.Name)
		if prevErr != nil {
			return prevErr
		}
		if len(execs) == 1 {
			gb.State.Executors = execs
			executorChosenFlag = true
		}
	}
	if !executorChosenFlag {
		err = gb.setExecutorsByParams(ctx, &setExecutorsByParamsDTO{
			Type:     params.Type,
			GroupID:  params.ExecutorsGroupID,
			Executor: params.Executors,
			WorkType: params.WorkType,
		})
		if err != nil {
			return err
		}
	}

	if notifyErr := gb.RunContext.handleInitiatorNotify(ctx, gb.Name, ef.TypeID, gb.GetTaskHumanStatus()); notifyErr != nil {
		return notifyErr
	}

	return gb.handleNotifications(ctx)
}

func (gb *GoExecutionBlock) loadState(raw json.RawMessage) error {
	return json.Unmarshal(raw, &gb.State)
}

//nolint:dupl,gocyclo //its not duplicate
func (gb *GoExecutionBlock) createState(ctx c.Context, ef *entity.EriusFunc) error {
	var params script.ExecutionParams
	err := json.Unmarshal(ef.Params, &params)
	if err != nil {
		return errors.Wrap(err, "can not get execution parameters")
	}

	if err = params.Validate(); err != nil {
		return errors.Wrap(err, "invalid execution parameters, work number")
	}

	gb.State = &ExecutionData{
		ExecutionType:      params.Type,
		CheckSLA:           params.CheckSLA,
		SLA:                params.SLA,
		ReworkSLA:          params.ReworkSLA,
		CheckReworkSLA:     params.CheckReworkSLA,
		FormsAccessibility: params.FormsAccessibility,
		IsEditable:         params.IsEditable,
		RepeatPrevDecision: params.RepeatPrevDecision,
		UseActualExecutor:  params.UseActualExecutor,
	}
	executorChosenFlag := false

	if params.ExecutorsGroupIDPath != nil && *params.ExecutorsGroupIDPath != "" {
		variableStorage, grabStorageErr := gb.RunContext.VarStore.GrabStorage()
		if grabStorageErr != nil {
			return grabStorageErr
		}

		groupId := getVariable(variableStorage, *params.ExecutorsGroupIDPath)
		if groupId == nil {
			return errors.New("can't find group id in variables")
		}
		params.ExecutorsGroupID = fmt.Sprintf("%v", groupId)
	}

	if gb.State.UseActualExecutor {
		execs, execErr := gb.RunContext.Storage.GetExecutorsFromPrevWorkVersionExecutionBlockRun(ctx, gb.RunContext.WorkNumber, gb.Name)
		if execErr != nil {
			return execErr
		}
		if len(execs) == 1 {
			gb.State.Executors = execs
			executorChosenFlag = true
		}
	}
	if !executorChosenFlag {
		err = gb.setExecutorsByParams(ctx, &setExecutorsByParamsDTO{
			Type:     params.Type,
			GroupID:  params.ExecutorsGroupID,
			Executor: params.Executors,
			WorkType: params.WorkType,
		})
		if err != nil {
			return err
		}
	}
	if params.WorkType != nil {
		gb.State.WorkType = *params.WorkType
	} else {
		task, getVersionErr := gb.RunContext.Storage.GetVersionByWorkNumber(ctx, gb.RunContext.WorkNumber)
		if getVersionErr != nil {
			return getVersionErr
		}

		processSLASettings, getVersionErr := gb.RunContext.Storage.GetSlaVersionSettings(ctx, task.VersionID.String())
		if getVersionErr != nil {
			return getVersionErr
		}
		gb.State.WorkType = processSLASettings.WorkType
	}

	if notifуErr := gb.RunContext.handleInitiatorNotify(ctx, gb.Name, ef.TypeID, gb.GetTaskHumanStatus()); notifуErr != nil {
		return notifуErr
	}

	return gb.handleNotifications(ctx)
}

type setExecutorsByParamsDTO struct {
	Type     script.ExecutionType
	GroupID  string
	Executor string
	WorkType *string
}

func (gb *GoExecutionBlock) setExecutorsByParams(ctx c.Context, dto *setExecutorsByParamsDTO) error {
	const variablesSep = ";"

	switch dto.Type {
	case script.ExecutionTypeUser:
		gb.State.Executors = map[string]struct{}{
			dto.Executor: {},
		}
		gb.State.IsTakenInWork = true
	case script.ExecutionTypeFromSchema:
		variableStorage, grabStorageErr := gb.RunContext.VarStore.GrabStorage()
		if grabStorageErr != nil {
			return grabStorageErr
		}

		executorsFromSchema := make(map[string]struct{})
		executorVars := strings.Split(dto.Executor, variablesSep)
		for i := range executorVars {
			resolvedEntities, resolveErr := getUsersFromVars(
				variableStorage,
				map[string]struct{}{
					executorVars[i]: {},
				},
			)
			if resolveErr != nil {
				return resolveErr
			}
			for executorLogin := range resolvedEntities {
				executorsFromSchema[executorLogin] = struct{}{}
			}
		}
		gb.State.Executors = executorsFromSchema
		if len(gb.State.Executors) == 1 {
			gb.State.IsTakenInWork = true
		}

	case script.ExecutionTypeGroup:
		workGroup, errGroup := gb.RunContext.ServiceDesc.GetWorkGroup(ctx, dto.GroupID)
		if errGroup != nil {
			return errors.Wrap(errGroup, "can`t get executors group with id: "+dto.GroupID)
		}

		if len(workGroup.People) == 0 {
			//nolint:goimports // bugged golint
			return errors.New("zero executors in group: " + dto.GroupID)
		}

		gb.State.Executors = make(map[string]struct{})
		for i := range workGroup.People {
			gb.State.Executors[workGroup.People[i].Login] = struct{}{}
		}
		gb.State.ExecutorsGroupID = dto.GroupID
		gb.State.ExecutorsGroupName = workGroup.GroupName
	}
	return nil
}

//nolint:unparam // ok here
func (gb *GoExecutionBlock) setPrevDecision(ctx c.Context) error {
	decision := gb.State.GetDecision()

	if decision == nil && len(gb.State.EditingAppLog) == 0 && gb.State.GetIsEditable() {
		gb.setEditingAppLogFromPreviousBlock(ctx)
	}

	if decision == nil && gb.State.GetRepeatPrevDecision() {
		if gb.trySetPreviousDecision(ctx) {
			return nil
		}
	}
	return nil
}

//nolint:dupl //its not duplicate
func (gb *GoExecutionBlock) setEditingAppLogFromPreviousBlock(ctx c.Context) {
	const funcName = "setEditingAppLogFromPreviousBlock"
	l := logger.GetLogger(ctx)

	var parentStep *entity.Step
	var err error

	parentStep, err = gb.RunContext.Storage.GetParentTaskStepByName(ctx, gb.RunContext.TaskID, gb.Name)
	if err != nil || parentStep == nil {
		return
	}

	// get state from step.State
	data, ok := parentStep.State[gb.Name]
	if !ok {
		l.Error(funcName, "step state is not found: "+gb.Name)
		return
	}

	var parentState ExecutionData
	if err = json.Unmarshal(data, &parentState); err != nil {
		l.Error(funcName, "invalid format of go-execution-block state")
		return
	}

	if len(parentState.EditingAppLog) > 0 {
		gb.State.EditingAppLog = parentState.EditingAppLog
	}
}

// nolint:dupl // not dupl
func (gb *GoExecutionBlock) trySetPreviousDecision(ctx c.Context) (isPrevDecisionAssigned bool) {
	const funcName = "pipeline.execution.trySetPreviousDecision"
	l := logger.GetLogger(ctx)

	var parentStep *entity.Step
	var err error

	parentStep, err = gb.RunContext.Storage.GetParentTaskStepByName(ctx, gb.RunContext.TaskID, gb.Name)
	if err != nil || parentStep == nil {
		l.Error(err)
		return false
	}

	data, ok := parentStep.State[gb.Name]
	if !ok {
		l.Error(funcName, "parent step state is not found: "+gb.Name)
		return false
	}

	var parentState ExecutionData
	if err = json.Unmarshal(data, &parentState); err != nil {
		l.Error(funcName, "invalid format of go-execution-block state")
		return false
	}

	if parentState.Decision != nil {
		var actualExecutor, comment string

		if parentState.ActualExecutor != nil {
			actualExecutor = *parentState.ActualExecutor
		}

		if parentState.DecisionComment != nil {
			comment = *parentState.DecisionComment
		}

		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputExecutionLogin], actualExecutor)
		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputDecision], &parentState.Decision)
		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputComment], comment)

		gb.State.ActualExecutor = &actualExecutor
		gb.State.DecisionComment = &comment
		gb.State.Decision = parentState.Decision
	}

	return true
}
