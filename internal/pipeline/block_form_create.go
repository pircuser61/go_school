package pipeline

import (
	c "context"
	"encoding/json"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

// nolint:dupl // another block
func createGoFormBlock(ctx c.Context, name string, ef *entity.EriusFunc, runCtx *BlockRunContext) (*GoFormBlock, error) {
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

	rawState, ok := runCtx.VarStore.State[name]
	if ok {
		if err := b.loadState(rawState); err != nil {
			return nil, err
		}

		if runCtx.UpdateData == nil || runCtx.UpdateData.Action == "" {
			if err := b.reEntry(ctx); err != nil {
				return nil, err
			}
		}
	} else {
		if err := b.createState(ctx, ef); err != nil {
			return nil, err
		}
		b.RunContext.VarStore.AddStep(b.Name)
	}

	return b, nil
}

func (gb *GoFormBlock) reEntry(ctx c.Context) error {
	if gb.State.RepeatPrevDecision {
		return nil
	}

	gb.State.IsFilled = false
	gb.State.IsTakenInWork = false
	gb.State.ActualExecutor = nil

	if gb.State.ReEnterSettings != nil {
		gb.State.CheckSLA = gb.State.ReEnterSettings.CheckSLA
		gb.State.SLA = gb.State.ReEnterSettings.SLA
		gb.State.FormExecutorType = gb.State.ReEnterSettings.FormExecutorType
		gb.State.FormGroupId = gb.State.ReEnterSettings.FormGroupId

		setErr := gb.setExecutorsByParams(ctx, &setFormExecutorsByParamsDTO{
			FormExecutorType: gb.State.ReEnterSettings.FormExecutorType,
			FormGroupId:      gb.State.ReEnterSettings.FormGroupId,
			Executor:         gb.State.ReEnterSettings.Executor,
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
		Executors: map[string]struct{}{
			params.Executor: {},
		},
		SchemaId:                  params.SchemaId,
		SLA:                       params.SLA,
		CheckSLA:                  params.CheckSLA,
		SchemaName:                params.SchemaName,
		ChangesLog:                make([]ChangesLogItem, 0),
		FormExecutorType:          params.FormExecutorType,
		ApplicationBody:           map[string]interface{}{},
		FormsAccessibility:        params.FormsAccessibility,
		Mapping:                   params.Mapping,
		HideExecutorFromInitiator: params.HideExecutorFromInitiator,
		RepeatPrevDecision:        params.RepeatPrevDecision,
		ReEnterSettings:           params.ReEnterSettings,
	}

	if setErr := gb.setExecutorsByParams(ctx, &setFormExecutorsByParamsDTO{}); setErr != nil {
		return setErr
	}

	return gb.handleNotifications(ctx)
}

type setFormExecutorsByParamsDTO struct {
	FormExecutorType script.FormExecutorType
	FormGroupId      string
	Executor         string
}

func (gb *GoFormBlock) setExecutorsByParams(ctx c.Context, dto *setFormExecutorsByParamsDTO) error {
	switch dto.FormExecutorType {
	case script.FormExecutorTypeUser:
		gb.State.Executors = map[string]struct{}{
			dto.Executor: {},
		}
	case script.FormExecutorTypeInitiator:
		gb.State.Executors = map[string]struct{}{
			gb.RunContext.Initiator: {},
		}
	case script.FormExecutorTypeFromSchema:
		variableStorage, grabStorageErr := gb.RunContext.VarStore.GrabStorage()
		if grabStorageErr != nil {
			return grabStorageErr
		}

		resolvedEntities, resolveErr := resolveValuesFromVariables(
			variableStorage,
			map[string]struct{}{
				dto.Executor: {},
			},
		)
		if resolveErr != nil {
			return resolveErr
		}

		gb.State.Executors = resolvedEntities
	case script.FormExecutorTypeAutoFillUser:
		if err := gb.handleAutoFillForm(); err != nil {
			return err
		}
	case script.FormExecutorTypeGroup:
		workGroup, errGroup := gb.RunContext.ServiceDesc.GetWorkGroup(ctx, dto.FormGroupId)
		if errGroup != nil {
			return errors.Wrap(errGroup, "can`t get form group with id: "+dto.FormGroupId)
		}

		if len(workGroup.People) == 0 {
			//nolint:goimports // bugged golint
			return errors.New("zero form executors in group: " + dto.FormGroupId)
		}

		gb.State.Executors = make(map[string]struct{})
		for i := range workGroup.People {
			gb.State.Executors[workGroup.People[i].Login] = struct{}{}
		}
		gb.State.FormGroupId = dto.FormGroupId
		gb.State.FormExecutorsGroupName = workGroup.GroupName
	}

	return nil
}
