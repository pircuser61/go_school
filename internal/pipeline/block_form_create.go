package pipeline

import (
	c "context"
	"encoding/json"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

// nolint:dupl // another block
func createGoFormBlock(ctx c.Context, name string, ef *entity.EriusFunc, runCtx *BlockRunContext) (*GoFormBlock, bool, error) {
	b := &GoFormBlock{
		Name:       name,
		Title:      ef.Title,
		Input:      map[string]string{},
		Output:     map[string]string{},
		Sockets:    entity.ConvertSocket(ef.Sockets),
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

		if reEntry {
			if err := b.reEntry(ctx); err != nil {
				return nil, false, err
			}
			b.RunContext.VarStore.AddStep(b.Name)
		}
	} else {
		if err := b.createState(ctx, ef); err != nil {
			return nil, false, err
		}
		b.RunContext.VarStore.AddStep(b.Name)
	}

	return b, reEntry, nil
}

func (gb *GoFormBlock) reEntry(ctx c.Context) error {
	if gb.State.IsEditable == nil || !*gb.State.IsEditable {
		return nil
	}

	gb.State.IsFilled = false
	gb.State.IsTakenInWork = false
	gb.State.ActualExecutor = nil

	if gb.State.ReEnterSettings != nil {
		setErr := gb.setExecutorsByParams(ctx, &setFormExecutorsByParamsDTO{
			FormExecutorType: gb.State.ReEnterSettings.FormExecutorType,
			Value:            gb.State.ReEnterSettings.Value,
		})
		if setErr != nil {
			return setErr
		}

		return gb.handleNotifications(ctx)
	}
	return nil
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

	gb.State = &FormData{
		SchemaId:                  params.SchemaId,
		CheckSLA:                  params.CheckSLA,
		SchemaName:                params.SchemaName,
		ChangesLog:                make([]ChangesLogItem, 0),
		FormExecutorType:          params.FormExecutorType,
		ApplicationBody:           map[string]interface{}{},
		FormsAccessibility:        params.FormsAccessibility,
		Mapping:                   params.Mapping,
		HideExecutorFromInitiator: params.HideExecutorFromInitiator,
		IsEditable:                params.IsEditable,
		ReEnterSettings:           params.ReEnterSettings,
	}

	executorValue := params.Executor
	if params.FormExecutorType == script.FormExecutorTypeGroup {
		executorValue = params.FormGroupId
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

	sla, getSLAErr := utils.GetAddressOfValue(WorkHourType(gb.State.WorkType)).GetTotalSLAInHours(params.SLA)

	if getSLAErr != nil {
		return getSLAErr
	}
	gb.State.SLA = sla

	return gb.handleNotifications(ctx)
}

type setFormExecutorsByParamsDTO struct {
	FormExecutorType script.FormExecutorType
	Value            string
}

func (gb *GoFormBlock) setExecutorsByParams(ctx c.Context, dto *setFormExecutorsByParamsDTO) error {
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

		resolvedEntities, resolveErr := resolveValuesFromVariables(
			variableStorage,
			map[string]struct{}{
				dto.Value: {},
			},
		)
		if resolveErr != nil {
			return resolveErr
		}

		gb.State.Executors = resolvedEntities
		gb.State.IsTakenInWork = true
	case script.FormExecutorTypeAutoFillUser:
		if err := gb.handleAutoFillForm(); err != nil {
			return err
		}
		gb.State.IsTakenInWork = true
	case script.FormExecutorTypeGroup:
		gb.State.FormGroupId = dto.Value
		workGroup, errGroup := gb.RunContext.ServiceDesc.GetWorkGroup(ctx, dto.Value)
		if errGroup != nil {
			return errors.Wrap(errGroup, "can`t get form group with id: "+dto.Value)
		}

		if len(workGroup.People) == 0 {
			//nolint:goimports // bugged golint
			return errors.New("zero form executors in group: " + dto.Value)
		}

		gb.State.Executors = make(map[string]struct{})
		for i := range workGroup.People {
			gb.State.Executors[workGroup.People[i].Login] = struct{}{}
		}
		gb.State.FormGroupId = dto.Value
		gb.State.FormExecutorsGroupName = workGroup.GroupName
	default:
		gb.State.Executors = map[string]struct{}{
			dto.Value: {},
		}
		gb.State.IsTakenInWork = true
	}

	return nil
}
