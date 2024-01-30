package pipeline

import (
	c "context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/jocasta/forms/pkg/jsonschema"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

// nolint:dupl // another block
func createGoFormBlock(
	ctx c.Context,
	name string,
	ef *entity.EriusFunc,
	runCtx *BlockRunContext,
	expectedEvents map[string]struct{},
) (*GoFormBlock, bool, error) {
	if ef.ShortTitle == "" {
		return nil, false, errors.New(ef.Title + " block short title is empty")
	}

	b := &GoFormBlock{
		Name:       name,
		ShortName:  ef.ShortTitle,
		Title:      ef.Title,
		Input:      map[string]string{},
		Output:     map[string]string{},
		Sockets:    entity.ConvertSocket(ef.Sockets),
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

	reEntry := false

	rawState, blockExists := runCtx.VarStore.State[name]
	if blockExists {
		if err := b.loadState(rawState); err != nil {
			return nil, false, err
		}

		reEntry = runCtx.UpdateData == nil

		if reEntry {
			if err := b.reEntry(ctx, ef); err != nil {
				return nil, false, err
			}

			b.RunContext.VarStore.AddStep(b.Name)

			if _, ok := b.expectedEvents[eventStart]; ok {
				status, _, _ := b.GetTaskHumanStatus()

				event, err := runCtx.MakeNodeStartEvent(ctx, MakeNodeStartEventArgs{
					NodeName:      name,
					NodeShortName: ef.ShortTitle,
					HumanStatus:   status,
					NodeStatus:    b.GetStatus(),
				})
				if err != nil {
					return nil, false, err
				}

				b.happenedEvents = append(b.happenedEvents, event)
			}
		}
	} else {
		if err := b.createState(ctx, ef); err != nil {
			return nil, false, err
		}

		b.RunContext.VarStore.AddStep(b.Name)
	}

	if _, ok := b.expectedEvents[eventStart]; ok {
		status, _, _ := b.GetTaskHumanStatus()

		event, err := runCtx.MakeNodeStartEvent(ctx, MakeNodeStartEventArgs{
			NodeName:      name,
			NodeShortName: ef.ShortTitle,
			HumanStatus:   status,
			NodeStatus:    b.GetStatus(),
		})
		if err != nil {
			return nil, false, err
		}

		b.happenedEvents = append(b.happenedEvents, event)
	}

	return b, reEntry, nil
}

func (gb *GoFormBlock) getHiddenFields(ctx c.Context, schemaID string) (res []string, err error) {
	var schema jsonschema.Schema

	schema, err = gb.RunContext.Services.ServiceDesc.GetSchemaByID(ctx, schemaID)
	if err != nil {
		return nil, err
	}

	if res, err = schema.GetHiddenFields(); err != nil {
		return nil, err
	}

	return res, nil
}

func (gb *GoFormBlock) reEntry(ctx c.Context, ef *entity.EriusFunc) error {
	if gb.State.IsEditable == nil || !*gb.State.IsEditable {
		return nil
	}

	isAutofill := gb.State.FormExecutorType == script.FormExecutorTypeAutoFillUser
	if isAutofill && gb.State.ReEnterSettings == nil {
		return errors.New("autofill with empty reenter settings data")
	}

	gb.State.IsFilled = false
	gb.State.IsTakenInWork = false
	gb.State.IsReentry = true
	gb.State.ActualExecutor = nil

	if !isAutofill && gb.State.FormExecutorType != script.FormExecutorTypeAutoFillUser {
		if gb.State.FormExecutorType == script.FormExecutorTypeFromSchema {
			var params script.FormParams

			err := json.Unmarshal(ef.Params, &params)
			if err != nil {
				return errors.Wrap(err, "can not get form parameters in reentry")
			}

			setErr := gb.setExecutorsByParams(ctx, &setFormExecutorsByParamsDTO{
				FormExecutorType: gb.State.FormExecutorType,
				Value:            params.Executor,
			})

			if setErr != nil {
				return setErr
			}
		} else {
			gb.State.Executors = gb.State.InitialExecutors
			gb.State.IsTakenInWork = len(gb.State.InitialExecutors) == 1
		}
	}

	if gb.State.FormExecutorType == script.FormExecutorTypeAutoFillUser && gb.State.ReEnterSettings != nil {
		err := gb.setReentryExecutors(ctx)
		if err != nil {
			return err
		}
	}

	return gb.handleNotifications(ctx)
}

func (gb *GoFormBlock) loadState(raw json.RawMessage) error {
	return json.Unmarshal(raw, &gb.State)
}

//nolint:dupl //different logic
func (gb *GoFormBlock) createState(ctx c.Context, ef *entity.EriusFunc) error {
	var params script.FormParams

	err := json.Unmarshal(ef.Params, &params)
	if err != nil {
		return errors.Wrap(err, "can not get form parameters")
	}

	if err = params.Validate(); err != nil {
		return errors.Wrap(err, "invalid form parameters")
	}

	hiddenFields, err := gb.getHiddenFields(ctx, params.SchemaID)
	if err != nil {
		return err
	}

	gb.State = &FormData{
		SchemaID:                  params.SchemaID,
		CheckSLA:                  params.CheckSLA,
		SLA:                       params.SLA,
		ChangesLog:                make([]ChangesLogItem, 0),
		FormExecutorType:          params.FormExecutorType,
		ApplicationBody:           map[string]interface{}{},
		FormsAccessibility:        params.FormsAccessibility,
		Mapping:                   params.Mapping,
		HideExecutorFromInitiator: params.HideExecutorFromInitiator,
		IsEditable:                params.IsEditable,
		ReEnterSettings:           params.ReEnterSettings,
		HiddenFields:              hiddenFields,
	}

	if params.FormGroupIDPath != nil && *params.FormGroupIDPath != "" {
		variableStorage, grabStorageErr := gb.RunContext.VarStore.GrabStorage()
		if grabStorageErr != nil {
			return grabStorageErr
		}

		groupID := getVariable(variableStorage, *params.FormGroupIDPath)
		if groupID == nil {
			return errors.New("can't find group id in variables")
		}

		params.FormGroupID = fmt.Sprintf("%v", groupID)
	}

	executorValue := params.Executor
	if params.FormExecutorType == script.FormExecutorTypeGroup {
		executorValue = params.FormGroupID
	}

	if setErr := gb.setExecutorsByParams(ctx, &setFormExecutorsByParamsDTO{
		FormExecutorType: params.FormExecutorType,
		Value:            executorValue,
	}); setErr != nil {
		return setErr
	}

	if params.WorkType != nil {
		gb.State.WorkType = *params.WorkType
	} else {
		task, getVersionErr := gb.RunContext.Services.Storage.GetVersionByWorkNumber(ctx, gb.RunContext.WorkNumber)
		if getVersionErr != nil {
			return getVersionErr
		}

		processSLASettings, getVersionErr := gb.RunContext.Services.Storage.GetSLAVersionSettings(ctx, task.VersionID.String())
		if getVersionErr != nil {
			return getVersionErr
		}
		gb.State.WorkType = processSLASettings.WorkType
	}

	return gb.handleNotifications(ctx)
}

type setFormExecutorsByParamsDTO struct {
	FormExecutorType script.FormExecutorType
	Value            string
}

func (gb *GoFormBlock) setExecutorsByParams(ctx c.Context, dto *setFormExecutorsByParamsDTO) error {
	const variablesSep = ";"

	// nolint:exhaustive //не хотим обрабатывать остальные случаи
	switch dto.FormExecutorType {
	case script.FormExecutorTypeInitiator:
		gb.State.Executors = map[string]struct{}{
			gb.RunContext.Initiator: {},
		}
		gb.State.IsTakenInWork = true
	case script.FormExecutorTypeFromSchema:
		variableStorage, grabStorageErr := gb.RunContext.VarStore.GrabStorage()
		if grabStorageErr != nil {
			return grabStorageErr
		}

		executorsFromSchema := make(map[string]struct{})
		executorVars := strings.Split(dto.Value, variablesSep)

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
	case script.FormExecutorTypeAutoFillUser:
		if err := gb.handleAutoFillForm(); err != nil {
			return err
		}

		gb.State.IsTakenInWork = true
	case script.FormExecutorTypeGroup:
		gb.State.FormGroupID = dto.Value

		workGroup, errGroup := gb.RunContext.Services.ServiceDesc.GetWorkGroup(ctx, dto.Value)
		if errGroup != nil {
			return errors.Wrap(errGroup, "can`t get form group with id: "+dto.Value)
		}

		if len(workGroup.People) == 0 {
			return errors.New("zero form executors in group: " + dto.Value)
		}

		gb.State.Executors = make(map[string]struct{}, len(workGroup.People))

		for i := range workGroup.People {
			gb.State.Executors[workGroup.People[i].Login] = struct{}{}
		}

		gb.State.FormGroupID = dto.Value
		gb.State.FormExecutorsGroupName = workGroup.GroupName
	default:
		gb.State.FormExecutorType = script.FormExecutorTypeUser
		gb.State.Executors = map[string]struct{}{
			dto.Value: {},
		}
		gb.State.IsTakenInWork = true
	}

	gb.State.InitialExecutors = gb.State.Executors

	return nil
}
