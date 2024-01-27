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
func createGoExecutionBlock(ctx c.Context, name string, ef *entity.EriusFunc, runCtx *BlockRunContext,
	expectedEvents map[string]struct{},
) (*GoExecutionBlock, bool, error) {
	if ef.ShortTitle == "" {
		return nil, false, errors.New(ef.Title + " block short title is empty")
	}

	b := &GoExecutionBlock{
		Name:      name,
		ShortName: ef.ShortTitle,
		Title:     ef.Title,
		Input:     map[string]string{},
		Output:    map[string]string{},
		Sockets:   entity.ConvertSocket(ef.Sockets),

		RunContext: runCtx,

		expectedEvents: expectedEvents,
		happenedEvents: make([]entity.NodeEvent, 0),
	}

	for _, v := range ef.Input {
		b.Input[v.Name] = v.Global
	}

	if ef.Output != nil {
		//nolint:gocritic //в этом проекте не принято использовать поинтеры в коллекциях
		for propertyName, v := range ef.Output.Properties {
			b.Output[propertyName] = v.Global
		}
	}

	rawState, blockExists := runCtx.VarStore.State[name]
	reEntry := false

	if blockExists {
		loadReEntry, err := b.load(ctx, rawState, runCtx, name, ef)
		if err != nil {
			return nil, false, err
		}

		reEntry = loadReEntry
	} else {
		err := b.init(ctx, runCtx, name, ef)
		if err != nil {
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
	gb.State.DecisionAttachments = make([]entity.Attachment, 0)
	gb.State.ActualExecutor = nil
	gb.State.IsTakenInWork = false
	gb.State.Executors = make(map[string]struct{})

	if gb.State.UseActualExecutor {
		execs, prevErr := gb.RunContext.Services.Storage.GetExecutorsFromPrevExecutionBlockRun(ctx, gb.RunContext.TaskID, gb.Name)
		if prevErr != nil {
			return prevErr
		}

		if len(execs) == 1 {
			gb.State.Executors = execs
		}
	}

	if len(gb.State.Executors) == 0 {
		err := gb.setExecutors(ctx, ef)
		if err != nil {
			return err
		}
	}

	return gb.handleNotifications(ctx)
}

func (gb *GoExecutionBlock) loadState(raw json.RawMessage) error {
	return json.Unmarshal(raw, &gb.State)
}

//nolint:dupl //its not duplicate
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
		HideExecutor:       params.HideExecutor,
	}

	if params.ExecutorsGroupIDPath != nil && *params.ExecutorsGroupIDPath != "" {
		variableStorage, grabStorageErr := gb.RunContext.VarStore.GrabStorage()
		if grabStorageErr != nil {
			return grabStorageErr
		}

		groupID := getVariable(variableStorage, *params.ExecutorsGroupIDPath)
		if groupID == nil {
			return errors.New("can't find group id in variables")
		}

		params.ExecutorsGroupID = fmt.Sprintf("%v", groupID)
	}

	if gb.State.UseActualExecutor {
		execs, execErr := gb.RunContext.Services.Storage.GetExecutorsFromPrevWorkVersionExecutionBlockRun(
			ctx,
			gb.RunContext.WorkNumber,
			gb.Name,
		)
		if execErr != nil {
			return execErr
		}

		if len(execs) == 1 {
			gb.State.Executors = execs
		}
	}

	if len(gb.State.Executors) == 0 {
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

		deadline, err := gb.getDeadline(ctx, gb.State.WorkType)
		if err != nil {
			return err
		}

		gb.State.Deadline = deadline
	} else {
		task, getVersionErr := gb.RunContext.Services.Storage.GetVersionByWorkNumber(ctx, gb.RunContext.WorkNumber)
		if getVersionErr != nil {
			return getVersionErr
		}

		processSLASettings, getVersionErr := gb.RunContext.Services.Storage.GetSLAVersionSettings(
			ctx, task.VersionID.String())
		if getVersionErr != nil {
			return getVersionErr
		}

		gb.State.WorkType = processSLASettings.WorkType
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
	case script.ExecutionTypeGroup:
		workGroup, errGroup := gb.RunContext.Services.ServiceDesc.GetWorkGroup(ctx, dto.GroupID)
		if errGroup != nil {
			return errors.Wrap(errGroup, "can`t get executors group with id: "+dto.GroupID)
		}

		if len(workGroup.People) == 0 {
			return errors.New("zero executors in group: " + dto.GroupID)
		}

		gb.State.Executors = make(map[string]struct{}, len(workGroup.People))

		for i := range workGroup.People {
			gb.State.Executors[workGroup.People[i].Login] = struct{}{}
		}

		gb.State.ExecutorsGroupID = dto.GroupID
		gb.State.ExecutorsGroupName = workGroup.GroupName
	}

	gb.State.InitialExecutors = gb.State.Executors

	return nil
}

func (gb *GoExecutionBlock) setPrevDecision(ctx c.Context) {
	decision := gb.State.GetDecision()

	isNeedSetPrevBlock := decision == nil && len(gb.State.EditingAppLog) == 0 && gb.State.GetIsEditable()
	if isNeedSetPrevBlock {
		gb.setEditingAppLogFromPreviousBlock(ctx)
	}

	if !gb.State.RepeatPrevDecision {
		return
	}

	if decision == nil {
		if gb.trySetPreviousDecision(ctx) {
			return
		}
	}

	if gb.State.UseActualExecutor || gb.State.Decision != nil {
		gb.setPreviousExecutors(ctx)
	}
}

//nolint:dupl //its not duplicate
func (gb *GoExecutionBlock) setEditingAppLogFromPreviousBlock(ctx c.Context) {
	const funcName = "setEditingAppLogFromPreviousBlock"

	l := logger.GetLogger(ctx)

	var (
		parentStep *entity.Step
		err        error
	)

	parentStep, err = gb.RunContext.Services.Storage.GetParentTaskStepByName(ctx, gb.RunContext.TaskID, gb.Name)
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

	log := logger.GetLogger(ctx)

	var (
		parentStep *entity.Step
		err        error
	)

	parentStep, err = gb.RunContext.Services.Storage.GetParentTaskStepByName(ctx, gb.RunContext.TaskID, gb.Name)
	if err != nil || parentStep == nil {
		log.Error(err)

		return false
	}

	data, ok := parentStep.State[gb.Name]
	if !ok {
		log.Error(funcName, "parent step state is not found: "+gb.Name)

		return false
	}

	var parentState ExecutionData
	if err = json.Unmarshal(data, &parentState); err != nil {
		log.Error(funcName, "invalid format of go-execution-block state")

		return false
	}

	if parentState.Decision != nil {
		err := gb.handleDecision(ctx, &parentState)
		if err != nil {
			return false
		}
	}

	return true
}

// nolint:dupl // not dupl
func (gb *GoExecutionBlock) setPreviousExecutors(ctx c.Context) {
	const funcName = "pipeline.execution.setPreviousExecutors"

	l := logger.GetLogger(ctx)

	var (
		parentStep *entity.Step
		err        error
	)

	parentStep, err = gb.RunContext.Services.Storage.GetParentTaskStepByName(ctx, gb.RunContext.TaskID, gb.Name)
	if err != nil || parentStep == nil {
		l.Error(err)

		return
	}

	data, ok := parentStep.State[gb.Name]
	if !ok {
		l.Error(funcName, "parent step state is not found: "+gb.Name)

		return
	}

	var parentState ExecutionData
	if err = json.Unmarshal(data, &parentState); err != nil {
		l.Error(funcName, "invalid format of go-execution-block state")

		return
	}

	if parentState.Executors != nil {
		gb.State.Executors = make(map[string]struct{}, len(parentState.Executors))
		for login := range parentState.Executors {
			gb.State.Executors[login] = struct{}{}
		}
	}
}
