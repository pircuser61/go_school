package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/jocasta/forms/pkg/jsonschema"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

const (
	titleKey      = "title"
	propertiesKey = "properties"
)

// nolint:dupl // another block
func createGoFormBlock(
	ctx context.Context,
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
		loadReEntry, err := b.load(ctx, rawState, runCtx, name, ef)
		if err != nil {
			return nil, false, err
		}

		reEntry = loadReEntry
	} else {
		if err := b.createState(ctx, ef); err != nil {
			return nil, false, err
		}

		b.RunContext.VarStore.AddStep(b.Name)

		err := b.makeNodeStartEventIfExpected(ctx, runCtx, name, ef)
		if err != nil {
			return nil, false, err
		}
	}

	isWorkOnEditing, err := b.RunContext.Services.Storage.CheckIsOnEditing(ctx, b.RunContext.TaskID.String())
	if err != nil {
		return nil, false, err
	}

	b.workIsOnEditing = isWorkOnEditing

	return b, reEntry, nil
}

func (gb *GoFormBlock) getHiddenFields(ctx context.Context, schemaID string) (res []string, err error) {
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

func (gb *GoFormBlock) load(
	ctx context.Context,
	rawState json.RawMessage,
	runCtx *BlockRunContext,
	name string,
	ef *entity.EriusFunc,
) (reEntry bool, err error) {
	err = gb.loadState(rawState)
	if err != nil {
		return false, err
	}

	reEntry = runCtx.UpdateData == nil

	err = gb.makeNodeStartEventWithReentry(ctx, reEntry, runCtx, name, ef)
	if err != nil {
		return false, err
	}

	return reEntry, nil
}

func (gb *GoFormBlock) makeNodeStartEventWithReentry(
	ctx context.Context,
	reEntry bool,
	runCtx *BlockRunContext,
	name string,
	ef *entity.EriusFunc,
) error {
	if reEntry {
		if err := gb.reEntry(ctx, ef); err != nil {
			return err
		}

		gb.RunContext.VarStore.AddStep(gb.Name)

		err := gb.makeNodeStartEventIfExpected(ctx, runCtx, name, ef)
		if err != nil {
			return err
		}
	}

	return nil
}

func (gb *GoFormBlock) makeNodeStartEventIfExpected(ctx context.Context, runCtx *BlockRunContext, name string, ef *entity.EriusFunc) error {
	if _, ok := gb.expectedEvents[eventStart]; ok {
		err := gb.makeNodeStartEvent(ctx, runCtx, name, ef)
		if err != nil {
			return err
		}
	}

	status, _, _ := gb.GetTaskHumanStatus()

	kafkaEvent, err := runCtx.MakeNodeKafkaEvent(ctx, &MakeNodeKafkaEvent{
		EventName:     eventStart,
		NodeName:      name,
		NodeShortName: ef.ShortTitle,
		HumanStatus:   status,
		NodeStatus:    gb.GetStatus(),
		NodeType:      BlockGoFormID,
		SLA:           gb.State.Deadline.Unix(),
		ToAddLogins:   getSliceFromMap(gb.State.Executors),
	})
	if err != nil {
		return err
	}

	gb.happenedKafkaEvents = append(gb.happenedKafkaEvents, kafkaEvent)

	return nil
}

func (gb *GoFormBlock) makeNodeStartEvent(ctx context.Context, runCtx *BlockRunContext, name string, ef *entity.EriusFunc) error {
	status, _, _ := gb.GetTaskHumanStatus()

	event, err := runCtx.MakeNodeStartEvent(ctx, MakeNodeStartEventArgs{
		NodeName:      name,
		NodeShortName: ef.ShortTitle,
		HumanStatus:   status,
		NodeStatus:    gb.GetStatus(),
	})
	if err != nil {
		return err
	}

	gb.happenedEvents = append(gb.happenedEvents, event)

	return err
}

func (gb *GoFormBlock) reEntry(ctx context.Context, ef *entity.EriusFunc) error {
	if gb.State.IsEditable == nil || !*gb.State.IsEditable {
		return nil
	}

	var params script.FormParams

	err := json.Unmarshal(ef.Params, &params)
	if err != nil {
		return errors.Wrap(err, "can not get form parameters in reentry")
	}

	isAutofill := gb.State.FormExecutorType == script.FormExecutorTypeAutoFillUser
	if isAutofill && gb.State.ReEnterSettings == nil {
		return errors.New("autofill with empty reenter settings data")
	}

	gb.State.IsFilled = false
	gb.State.IsTakenInWork = false
	gb.State.IsReentry = true
	gb.State.ActualExecutor = nil
	gb.State.IsExpired = false

	if !isAutofill && gb.State.FormExecutorType != script.FormExecutorTypeAutoFillUser {
		err := gb.setExecutors(ctx, &params)
		if err != nil {
			return err
		}
	}

	if gb.State.FormExecutorType == script.FormExecutorTypeAutoFillUser && gb.State.ReEnterSettings != nil {
		err := gb.setReentryExecutors(ctx)
		if err != nil {
			return err
		}
	}

	if deadlineErr := gb.setWorkTypeAndDeadline(ctx, &params); deadlineErr != nil {
		return deadlineErr
	}

	return gb.handleNotifications(ctx)
}

func (gb *GoFormBlock) setExecutors(ctx context.Context, params *script.FormParams) error {
	if gb.State.FormExecutorType == script.FormExecutorTypeFromSchema {
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

	return nil
}

func (gb *GoFormBlock) setReentryExecutors(ctx context.Context) error {
	if gb.State.ReEnterSettings.GroupPath != nil && *gb.State.ReEnterSettings.GroupPath != "" {
		variableStorage, grabStorageErr := gb.RunContext.VarStore.GrabStorage()
		if grabStorageErr != nil {
			return grabStorageErr
		}

		groupID := getVariable(variableStorage, *gb.State.ReEnterSettings.GroupPath)
		if groupID == nil {
			return errors.New("can't find group id in variables")
		}

		gb.State.ReEnterSettings.Value = fmt.Sprintf("%v", groupID)
	}

	setErr := gb.setExecutorsByParams(
		ctx,
		&setFormExecutorsByParamsDTO{
			FormExecutorType: gb.State.ReEnterSettings.FormExecutorType,
			Value:            gb.State.ReEnterSettings.Value,
		},
	)
	if setErr != nil {
		return setErr
	}

	gb.State.FormExecutorType = gb.State.ReEnterSettings.FormExecutorType

	return nil
}

func (gb *GoFormBlock) loadState(raw json.RawMessage) error {
	return json.Unmarshal(raw, &gb.State)
}

//nolint:dupl //different logic
func (gb *GoFormBlock) createState(ctx context.Context, ef *entity.EriusFunc) error {
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
		return errors.Wrap(err, "can`t get hidden fields")
	}

	schema, err := gb.RunContext.Services.ServiceDesc.GetSchemaByID(ctx, params.SchemaID)
	if err != nil {
		return errors.Wrap(err, "can`t get schema by ID")
	}

	schema = checkFormGroup(schema)
	if prop, ok := schema[propertiesKey]; ok {
		propMap, propOk := prop.(map[string]interface{})
		if !propOk {
			return errors.New("properties is not map")
		}

		schemaJSON := jsonschema.Schema(propMap)

		keys, _, getAllFieldsErr := schemaJSON.GetAllFields()
		if getAllFieldsErr != nil {
			return getAllFieldsErr
		}

		params.Keys = keys

		params.AttachmentFields = schemaJSON.GetAttachmentFields()
	}

	gb.State = &FormData{
		SchemaID:                  params.SchemaID,
		CheckSLA:                  params.CheckSLA,
		SLA:                       params.SLA,
		ChangesLog:                make([]ChangesLogItem, 0),
		FormExecutorType:          params.FormExecutorType,
		ApplicationBody:           map[string]interface{}{},
		Constants:                 params.Constants,
		FormsAccessibility:        params.FormsAccessibility,
		Mapping:                   params.Mapping,
		FullFormMapping:           params.FullFormMapping,
		HideExecutorFromInitiator: params.HideExecutorFromInitiator,
		IsEditable:                params.IsEditable,
		CheckRequiredForm:         params.CheckRequiredForm,
		ReEnterSettings:           params.ReEnterSettings,
		HiddenFields:              hiddenFields,
		Keys:                      params.Keys,
		AttachmentFields:          params.AttachmentFields,
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

	if deadlineErr := gb.setWorkTypeAndDeadline(ctx, &params); deadlineErr != nil {
		return deadlineErr
	}

	return gb.handleNotifications(ctx)
}

func (gb *GoFormBlock) setWorkTypeAndDeadline(ctx context.Context, params *script.FormParams) error {
	if params.WorkType != nil {
		gb.State.WorkType = *params.WorkType
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

	deadline, err := gb.getDeadline(ctx, gb.State.WorkType)
	if err != nil {
		return err
	}

	gb.State.Deadline = deadline

	return nil
}

type setFormExecutorsByParamsDTO struct {
	FormExecutorType script.FormExecutorType
	Value            string
}

func (gb *GoFormBlock) setExecutorsByParams(ctx context.Context, dto *setFormExecutorsByParamsDTO) error {
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

func checkFormGroup(rawStartSchema map[string]interface{}) jsonschema.Schema {
	properties, ok := rawStartSchema[propertiesKey]
	if !ok {
		return rawStartSchema
	}

	propertiesMap := properties.(map[string]interface{})

	for k, v := range propertiesMap {
		valMap, mapOk := v.(map[string]interface{})
		if !mapOk {
			continue
		}

		if valMap[titleKey] == "" {
			valMap[titleKey] = " "
		} else {
			newTitle := cleanKey(v)
			if newTitle != "" {
				valMap[titleKey] = newTitle
			}
		}

		propVal, propValOk := valMap[propertiesKey]
		if !propValOk {
			continue
		}

		propValMap := propVal.(map[string]interface{})
		for key, val := range propValMap {
			valMaps, mapOks := v.(map[string]interface{})
			if mapOks {
				propertiesMap[key] = val

				continue
			}

			if valMaps[titleKey] == "" {
				valMap[titleKey] = " "
			} else {
				newAdTitle := cleanKey(val)
				if newAdTitle != "" {
					valMaps[titleKey] = newAdTitle
				}
			}

			propVals, propValOks := valMaps[propertiesKey]
			if !propValOks {
				continue
			}

			propValMaps := propVals.(map[string]interface{})
			for keys, vals := range propValMaps {
				propertiesMap[keys] = vals
			}
		}

		delete(propertiesMap, k)
	}

	return rawStartSchema
}

func cleanKey(mapKeys interface{}) string {
	keys, ok := mapKeys.(map[string]interface{})
	if !ok {
		return ""
	}

	key, oks := keys[titleKey]
	if !oks {
		return ""
	}

	replacements := map[string]string{
		"\\t":  "",
		"\t":   "",
		"\\n":  "",
		"\n":   "",
		"\r":   "",
		"\\r":  "",
		"\"\"": "",
	}

	keyStr, okStr := key.(string)
	if !okStr {
		return ""
	}

	for old, news := range replacements {
		keyStr = strings.ReplaceAll(keyStr, old, news)
	}

	return strings.ReplaceAll(keyStr, "\\", "")
}
