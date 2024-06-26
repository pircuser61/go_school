package api

import (
	c "context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"reflect"
	"strings"

	"github.com/google/uuid"
	"gitlab.services.mts.ru/abp/myosotis/logger"
	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/user"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

//nolint:revive,gocritic,stylecheck
func (ae *Env) MonitoringUpdateTaskBlockData(w http.ResponseWriter, r *http.Request, blockId string) {
	const fn = "MonitoringUpdateTaskBlockData"

	ctx, span := trace.StartSpan(r.Context(), "monitoring_update_task_block_data")
	defer span.End()

	log := logger.GetLogger(ctx).
		WithField("funcName", fn).
		WithField("stepID", blockId)
	errorHandler := newHTTPErrorHandler(log, w)

	b, err := io.ReadAll(r.Body)

	defer r.Body.Close()

	if err != nil {
		errorHandler.handleError(RequestReadError, err)

		return
	}

	txStorage, transactionErr := ae.DB.StartTransaction(ctx)
	if transactionErr != nil {
		log.WithError(transactionErr).Error("couldn't start transaction")

		errorHandler.sendError(UnknownError)

		return
	}

	req := &MonitoringTaskUpdateBlockRequest{}

	err = json.Unmarshal(b, req)
	if err != nil {
		errorHandler.handleError(MonitoringEditBlockParseError, err)
		ae.rollbackTransaction(ctx, txStorage, fn)

		return
	}

	data, err := convertReqEditData(req.ChangeData.AdditionalProperties)
	if err != nil {
		log.Error(fmt.Errorf("type and type of value are not compatible: %w", err))
		errorHandler.handleError(TypeAndValueNotCompatible, err)

		return
	}

	blockID, parseIDErr := uuid.Parse(blockId)
	if parseIDErr != nil {
		errorHandler.handleError(UUIDParsingError, parseIDErr)
		ae.rollbackTransaction(ctx, txStorage, fn)

		return
	}

	dbStep, getStepErr := ae.DB.GetTaskStepByID(ctx, blockID)
	if getStepErr != nil {
		errorHandler.handleError(GetTaskStepError, getStepErr)
		ae.rollbackTransaction(ctx, txStorage, fn)

		return
	}

	log = log.WithField("stepName", dbStep.Name).
		WithField("workID", dbStep.WorkID).
		WithField("workNumber", dbStep.WorkNumber)
	ctx = logger.WithLogger(ctx, log)

	editBlockData, err := ae.editGoBlock(ctx, &editGoBlockDTO{
		stepID:     blockID,
		stepType:   dbStep.Type,
		stepName:   dbStep.Name,
		data:       data,
		updateType: req.ChangeType,
	})
	if err != nil {
		errorHandler.handleError(EditMonitoringBlockError, err)
		ae.rollbackTransaction(ctx, txStorage, fn)

		return
	}

	ui, err := user.GetUserInfoFromCtx(ctx)
	if err != nil {
		errorHandler.handleError(NoUserInContextError, err)
		ae.rollbackTransaction(ctx, txStorage, fn)

		return
	}

	eventData := struct {
		Data       map[string]interface{} `json:"data"`
		Steps      []uuid.UUID            `json:"steps"`
		ChangeType string                 `json:"change_type"`
	}{
		Data:       data,
		ChangeType: string(req.ChangeType),
		Steps:      []uuid.UUID{blockID},
	}

	// nolint:ineffassign,staticcheck
	jsonParams := json.RawMessage{}

	jsonParams, err = json.Marshal(eventData)
	if err != nil {
		errorHandler.handleError(MarshalEventParamsError, err)
		ae.rollbackTransaction(ctx, txStorage, fn)

		return
	}

	eventID, err := txStorage.CreateTaskEvent(ctx, &entity.CreateTaskEvent{
		WorkID:    dbStep.WorkID.String(),
		Author:    ui.Username,
		EventType: string(MonitoringTaskActionRequestActionEdit),
		Params:    jsonParams,
	})
	if err != nil {
		errorHandler.handleError(CreateTaskEventError, err)
		ae.rollbackTransaction(ctx, txStorage, fn)

		return
	}

	updErr := ae.UpdateContent(ctx, txStorage, editBlockData, dbStep.WorkID.String(), eventID)
	if updErr != -1 {
		errorHandler.sendError(updErr)
		ae.rollbackTransaction(ctx, txStorage, fn)

		return
	}

	err = txStorage.CommitTransaction(ctx)
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}

	var getErr Err = -1

	switch req.ChangeType {
	case MonitoringTaskUpdateBlockRequestChangeTypeContext:
		getErr = ae.returnContext(ctx, blockId, w)
	case MonitoringTaskUpdateBlockRequestChangeTypeOutput:
		getErr = ae.returnOutput(ctx, blockId, w, dbStep)
	case MonitoringTaskUpdateBlockRequestChangeTypeState:
		getErr = ae.returnState(ctx, blockId, w)
	}

	if getErr != -1 {
		errorHandler.sendError(getErr)
	}
}

func convertReqEditData(in map[string]MonitoringEditBlockData) (res map[string]interface{}, err error) {
	res = map[string]interface{}{}

	for key, val := range in {
		err = IsTypeCorrect(val.Type, val.Value)
		if err != nil {
			return nil, err
		}

		res[key] = val.Value
	}

	return res, nil
}

func IsTypeCorrect(t string, v any) error {
	if v == nil && t == "" {
		return nil
	} else if t == "" {
		return fmt.Errorf("empty type")
	}

	if v == nil && (t == utils.ObjectType || t == utils.ArrayType) {
		return nil
	} else if v == nil {
		return fmt.Errorf("value of type %s can't be null", t)
	}

	reflectType := reflect.TypeOf(v).Kind()

	typeIsCorrect := false

	switch t {
	case utils.IntegerType:
		if reflectType == reflect.Float64 {
			floatNum, ok := v.(float64)
			if ok {
				typeIsCorrect = checkIfInteger(floatNum)
			}
		} else if reflectType == reflect.Int {
			typeIsCorrect = (reflectType == reflect.Int)
		}
	case utils.NumberType:
		typeIsCorrect = (reflectType == reflect.Float64)
	case utils.StringType:
		typeIsCorrect = (reflectType == reflect.String)
	case utils.BoolType:
		typeIsCorrect = (reflectType == reflect.Bool)
	case utils.ArrayType:
		typeIsCorrect = (reflectType == reflect.Slice)
	case utils.ObjectType:
		typeIsCorrect = (reflectType == reflect.Map)
	}

	if typeIsCorrect {
		return nil
	}

	return fmt.Errorf("not compatible: type is %s, type of value is %s", t, reflectType.String())
}

func checkIfInteger(a float64) bool {
	return (a - math.Floor(a)) == 0
}

func (ae *Env) rollbackTransaction(ctx c.Context, tx db.Database, fn string) {
	rollbackErr := tx.RollbackTransaction(ctx)
	if rollbackErr != nil {
		ae.Log.WithError(rollbackErr).
			WithField("funcName", fn).
			Error("failed rollback transaction")
	}
}

func (ae *Env) UpdateContent(ctx c.Context, txStorage db.Database, data []EditBlock, workID, eventID string) (err Err) {
	for i := range data {
		savePrevErr := txStorage.CreateStepPreviousContent(ctx, data[i].StepID.String(), eventID)
		if savePrevErr != nil {
			return SaveNodePrevContentError
		}
	}

	for i := range data {
		saveErr := txStorage.UpdateStepContent(ctx, data[i].StepID.String(), workID, data[i].StepName,
			data[i].State, data[i].Output)
		if saveErr != nil {
			return SaveUpdatedBlockData
		}
	}

	return -1
}

func (ae *Env) returnState(ctx c.Context, blockID string, w http.ResponseWriter) (getErr Err) {
	state, err := ae.DB.GetBlockStateForMonitoring(ctx, blockID)
	if err != nil {
		return GetBlockStateError
	}

	params := make(map[string]MonitoringEditBlockData, len(state))
	for _, bo := range state {
		params[bo.Name] = MonitoringEditBlockData{
			Name:  bo.Name,
			Value: bo.Value,
			Type:  utils.GetJSONType(bo.Value),
		}
	}

	if err = sendResponse(w, http.StatusOK, BlockEditResponse{
		Blocks: &BlockEditResponse_Blocks{params},
	}); err != nil {
		return UnknownError
	}

	return -1
}

func (ae *Env) returnContext(ctx c.Context, blockID string, w http.ResponseWriter) (getErr Err) {
	blocksOutputs, err := ae.DB.GetBlocksOutputs(ctx, blockID)
	if err != nil {
		return GetBlockContextError
	}

	blocks := make(map[string]MonitoringEditBlockData, len(blocksOutputs))

	for _, bo := range blocksOutputs {
		prefix := bo.StepName + "."

		if strings.HasPrefix(bo.Name, prefix) {
			continue
		}

		blocks[bo.Name] = MonitoringEditBlockData{
			Name:        bo.Name,
			Value:       bo.Value,
			Description: "",
			Type:        utils.GetJSONType(bo.Value),
		}
	}

	err = sendResponse(w, http.StatusOK, BlockEditResponse{
		Blocks: &BlockEditResponse_Blocks{blocks},
	})
	if err != nil {
		return UnknownError
	}

	return -1
}

func (ae *Env) returnOutput(ctx c.Context, blockID string, w http.ResponseWriter, step *entity.Step) (getErr Err) {
	blockOutputs, err := ae.DB.GetBlockOutputs(ctx, blockID, step.Name)
	if err != nil {
		return GetBlockContextError
	}

	outputs := make(map[string]MonitoringEditBlockData, 0)

	for _, bo := range blockOutputs {
		outputs[bo.Name] = MonitoringEditBlockData{
			Name:  bo.Name,
			Value: bo.Value,
			Type:  utils.GetJSONType(bo.Value),
		}
	}

	if err := sendResponse(w, http.StatusOK, BlockEditResponse{
		Blocks: &BlockEditResponse_Blocks{AdditionalProperties: outputs},
	}); err != nil {
		return UnknownError
	}

	return -1
}

type EditBlock struct {
	StepID        uuid.UUID
	StepName      string
	State, Output map[string]interface{}
}

type editGoBlockDTO struct {
	stepID     uuid.UUID
	stepType   string
	stepName   string
	data       map[string]interface{}
	updateType MonitoringTaskUpdateBlockRequestChangeType
}

func (ae *Env) editGoBlock(ctx c.Context, in *editGoBlockDTO) (res []EditBlock, err error) {
	switch in.stepType {
	case pipeline.BlockGoApproverID:
		res, err = ae.approverEditBlock(ctx, in.stepID, in.stepName, in.data, in.updateType)
	case pipeline.BlockGoExecutionID:
		res, err = ae.executorEditBlock(ctx, in.stepID, in.stepName, in.data, in.updateType)
	case pipeline.BlockGoStartID:
		res, err = ae.startEditBlock(ctx, in.stepID, in.stepName, in.data, in.updateType)
	case pipeline.BlockGoEndID:
		res, err = ae.endEditBlock(ctx, in.stepID, in.stepName, in.data, in.updateType)
	case pipeline.BlockGoBeginParallelTaskID:
		res, err = ae.startParallelEditBlock(ctx, in.stepID, in.stepName, in.data, in.updateType)
	case pipeline.BlockWaitForAllInputsID:
		res, err = ae.endParallelEditBlock(ctx, in.stepID, in.stepName, in.data, in.updateType)
	case pipeline.BlockExecutableFunctionID:
		res, err = ae.functionEditBlock(ctx, in.stepID, in.stepName, in.data, in.updateType)
	case pipeline.BlockGoFormID:
		res, err = ae.formEditBlock(ctx, in.stepID, in.stepName, in.data, in.updateType)
	case pipeline.BlockGoIfID:
		res, err = ae.ifEditBlock(ctx, in.stepID, in.stepName, in.data, in.updateType)
	case pipeline.BlockGoNotificationID:
		res, err = ae.notificationEditBlock(ctx, in.stepID, in.stepName, in.data, in.updateType)
	case pipeline.BlockGoSdApplicationID:
		res, err = ae.sdEditBlock(ctx, in.stepID, in.stepName, in.data, in.updateType)
	case pipeline.BlockGoSignID:
		res, err = ae.signEditBlock(ctx, in.stepID, in.stepName, in.data, in.updateType)
	case pipeline.BlockTimerID:
		res, err = ae.timerEditBlock(ctx, in.stepID, in.stepName, in.data, in.updateType)
	default:
		err = fmt.Errorf("unknown block type %s", in.stepType)
	}

	return res, err
}

// nolint:dupl //duplicate is ok here
func (ae *Env) approverEditBlock(ctx c.Context, stepID uuid.UUID, stepName string, data map[string]interface{},
	updateType MonitoringTaskUpdateBlockRequestChangeType,
) (res []EditBlock, err error) {
	marshData, marshErr := json.Marshal(data)
	if marshErr != nil {
		return nil, marshErr
	}

	switch updateType {
	case MonitoringTaskUpdateBlockRequestChangeTypeContext:
		return ae.editBlockContext(ctx, stepID, data)
	case MonitoringTaskUpdateBlockRequestChangeTypeOutput:
		blockState, stateErr := ae.DB.GetBlockState(ctx, stepID.String())
		if stateErr != nil {
			return nil, stateErr
		}

		approverState := pipeline.ApproverData{
			Approvers:           make(map[string]struct{}, 0),
			ApproverLog:         make([]pipeline.ApproverLogEntry, 0),
			EditingAppLog:       make([]pipeline.ApproverEditingApp, 0),
			FormsAccessibility:  make([]script.FormAccessibility, 0),
			AddInfo:             make([]pipeline.AdditionalInfo, 0),
			ActionList:          make([]pipeline.Action, 0),
			AdditionalApprovers: make([]pipeline.AdditionalApprover, 0),
		}

		unmErr := json.Unmarshal(blockState, &approverState)
		if unmErr != nil {
			return nil, unmErr
		}

		block := pipeline.GoApproverBlock{State: &approverState}

		updState, updErr := block.UpdateStateUsingOutput(ctx, marshData)
		if updErr != nil {
			return nil, updErr
		}

		return []EditBlock{{State: updState, Output: data, StepName: stepName, StepID: stepID}}, nil

	case MonitoringTaskUpdateBlockRequestChangeTypeState:
		approverState := pipeline.ApproverData{
			Approvers:           make(map[string]struct{}, 0),
			ApproverLog:         make([]pipeline.ApproverLogEntry, 0),
			EditingAppLog:       make([]pipeline.ApproverEditingApp, 0),
			FormsAccessibility:  make([]script.FormAccessibility, 0),
			AddInfo:             make([]pipeline.AdditionalInfo, 0),
			ActionList:          make([]pipeline.Action, 0),
			AdditionalApprovers: make([]pipeline.AdditionalApprover, 0),
		}

		unmErr := json.Unmarshal(marshData, &approverState)
		if unmErr != nil {
			return nil, unmErr
		}

		block := pipeline.GoApproverBlock{
			State:      &approverState,
			RunContext: &pipeline.BlockRunContext{Services: pipeline.RunContextServices{ServiceDesc: ae.ServiceDesc}},
		}

		updOutput, updErr := block.UpdateOutputUsingState(ctx)
		if updErr != nil {
			return nil, updErr
		}

		return []EditBlock{{State: data, Output: updOutput, StepName: stepName, StepID: stepID}}, nil
	}

	return res, nil
}

// nolint:dupl //duplicate is ok here
func (ae *Env) executorEditBlock(ctx c.Context, stepID uuid.UUID, stepName string, data map[string]interface{},
	updateType MonitoringTaskUpdateBlockRequestChangeType,
) (res []EditBlock, err error) {
	marshData, marshErr := json.Marshal(data)
	if marshErr != nil {
		return nil, marshErr
	}

	switch updateType {
	case MonitoringTaskUpdateBlockRequestChangeTypeContext:
		return ae.editBlockContext(ctx, stepID, data)
	case MonitoringTaskUpdateBlockRequestChangeTypeOutput:
		blockState, stateErr := ae.DB.GetBlockState(ctx, stepID.String())
		if stateErr != nil {
			return nil, stateErr
		}

		execState := pipeline.ExecutionData{
			Executors:                make(map[string]struct{}, 0),
			InitialExecutors:         make(map[string]struct{}, 0),
			DecisionAttachments:      make([]entity.Attachment, 0),
			EditingAppLog:            make([]pipeline.ExecutorEditApp, 0),
			ChangedExecutorsLogs:     make([]pipeline.ChangeExecutorLog, 0),
			RequestExecutionInfoLogs: make([]pipeline.RequestExecutionInfoLog, 0),
			FormsAccessibility:       make([]script.FormAccessibility, 0),
			TakenInWorkLog:           make([]pipeline.StartWorkLog, 0),
		}

		unmErr := json.Unmarshal(blockState, &execState)
		if unmErr != nil {
			return nil, unmErr
		}

		block := pipeline.GoExecutionBlock{State: &execState}

		updState, updErr := block.UpdateStateUsingOutput(ctx, marshData)
		if updErr != nil {
			return nil, updErr
		}

		return []EditBlock{{State: updState, Output: data, StepName: stepName, StepID: stepID}}, nil
	case MonitoringTaskUpdateBlockRequestChangeTypeState:
		execState := pipeline.ExecutionData{
			Executors:                make(map[string]struct{}, 0),
			InitialExecutors:         make(map[string]struct{}, 0),
			DecisionAttachments:      make([]entity.Attachment, 0),
			EditingAppLog:            make([]pipeline.ExecutorEditApp, 0),
			ChangedExecutorsLogs:     make([]pipeline.ChangeExecutorLog, 0),
			RequestExecutionInfoLogs: make([]pipeline.RequestExecutionInfoLog, 0),
			FormsAccessibility:       make([]script.FormAccessibility, 0),
			TakenInWorkLog:           make([]pipeline.StartWorkLog, 0),
		}

		unmErr := json.Unmarshal(marshData, &execState)
		if unmErr != nil {
			return nil, unmErr
		}

		block := pipeline.GoExecutionBlock{
			State:      &execState,
			RunContext: &pipeline.BlockRunContext{Services: pipeline.RunContextServices{ServiceDesc: ae.ServiceDesc}},
		}

		updOutput, updErr := block.UpdateOutputUsingState(ctx)
		if updErr != nil {
			return nil, updErr
		}

		return []EditBlock{{State: data, Output: updOutput, StepName: stepName, StepID: stepID}}, nil
	}

	return res, nil
}

// nolint:dupl //duplicate is ok here
func (ae *Env) startEditBlock(ctx c.Context, stepID uuid.UUID, stepName string, data map[string]interface{},
	updateType MonitoringTaskUpdateBlockRequestChangeType,
) (res []EditBlock, err error) {
	switch updateType {
	case MonitoringTaskUpdateBlockRequestChangeTypeContext:
		return ae.editBlockContext(ctx, stepID, data)
	case MonitoringTaskUpdateBlockRequestChangeTypeOutput:
		return []EditBlock{{State: map[string]interface{}{}, Output: data, StepName: stepName, StepID: stepID}}, nil

	case MonitoringTaskUpdateBlockRequestChangeTypeState:
		blockOutputs, stateErr := ae.DB.GetBlockOutputs(ctx, stepID.String(), stepName)
		if stateErr != nil {
			return nil, stateErr
		}

		startOutputs := make(map[string]interface{})

		for i := range blockOutputs {
			output := blockOutputs[i]
			startOutputs[output.Name] = output.Value
		}

		return []EditBlock{{State: data, Output: startOutputs, StepName: stepName, StepID: stepID}}, nil
	}

	return res, nil
}

// nolint:dupl //duplicate is ok here
func (ae *Env) endEditBlock(ctx c.Context, stepID uuid.UUID, stepName string, data map[string]interface{},
	updateType MonitoringTaskUpdateBlockRequestChangeType,
) (res []EditBlock, err error) {
	switch updateType {
	case MonitoringTaskUpdateBlockRequestChangeTypeContext:
		return ae.editBlockContext(ctx, stepID, data)
	case MonitoringTaskUpdateBlockRequestChangeTypeOutput:
		return []EditBlock{{State: map[string]interface{}{}, Output: data, StepName: stepName, StepID: stepID}}, nil
	case MonitoringTaskUpdateBlockRequestChangeTypeState:
		return []EditBlock{{State: data, Output: map[string]interface{}{}, StepName: stepName, StepID: stepID}}, nil
	}

	return res, nil
}

// nolint:dupl //duplicate is ok here
func (ae *Env) startParallelEditBlock(ctx c.Context, stepID uuid.UUID, stepName string, data map[string]interface{},
	updateType MonitoringTaskUpdateBlockRequestChangeType,
) (res []EditBlock, err error) {
	switch updateType {
	case MonitoringTaskUpdateBlockRequestChangeTypeContext:
		return ae.editBlockContext(ctx, stepID, data)
	case MonitoringTaskUpdateBlockRequestChangeTypeOutput:
		return []EditBlock{{State: map[string]interface{}{}, Output: data, StepName: stepName, StepID: stepID}}, nil

	case MonitoringTaskUpdateBlockRequestChangeTypeState:
		return []EditBlock{{State: data, Output: map[string]interface{}{}, StepName: stepName, StepID: stepID}}, nil
	}

	return res, nil
}

// nolint:dupl //duplicate is ok here
func (ae *Env) endParallelEditBlock(ctx c.Context, stepID uuid.UUID, stepName string, data map[string]interface{},
	updateType MonitoringTaskUpdateBlockRequestChangeType,
) (res []EditBlock, err error) {
	switch updateType {
	case MonitoringTaskUpdateBlockRequestChangeTypeContext:
		return ae.editBlockContext(ctx, stepID, data)
	case MonitoringTaskUpdateBlockRequestChangeTypeOutput:
		blockState, stateErr := ae.DB.GetBlockState(ctx, stepID.String())
		if stateErr != nil {
			return nil, stateErr
		}

		endParallelState := make(map[string]interface{})

		unmErr := json.Unmarshal(blockState, &endParallelState)
		if unmErr != nil {
			return nil, unmErr
		}

		return []EditBlock{{State: endParallelState, Output: data, StepName: stepName, StepID: stepID}}, nil
	case MonitoringTaskUpdateBlockRequestChangeTypeState:
		return []EditBlock{{State: data, Output: map[string]interface{}{}, StepName: stepName, StepID: stepID}}, nil
	}

	return res, nil
}

// nolint:dupl //duplicate is ok here
func (ae *Env) functionEditBlock(ctx c.Context, stepID uuid.UUID, stepName string, data map[string]interface{},
	updateType MonitoringTaskUpdateBlockRequestChangeType,
) (res []EditBlock, err error) {
	marshData, marshErr := json.Marshal(data)
	if marshErr != nil {
		return nil, marshErr
	}

	switch updateType {
	case MonitoringTaskUpdateBlockRequestChangeTypeContext:
		return ae.editBlockContext(ctx, stepID, data)
	case MonitoringTaskUpdateBlockRequestChangeTypeOutput:
		blockState, stateErr := ae.DB.GetBlockState(ctx, stepID.String())
		if stateErr != nil {
			return nil, stateErr
		}

		funcState := pipeline.ExecutableFunction{
			Mapping:       make(map[string]script.JSONSchemaPropertiesValue, 0),
			Constants:     make(map[string]interface{}, 0),
			RetryTimeouts: make([]int, 0),
		}

		unmErr := json.Unmarshal(blockState, &funcState)
		if unmErr != nil {
			return nil, unmErr
		}

		block := pipeline.ExecutableFunctionBlock{State: &funcState}

		updState, updErr := block.UpdateStateUsingOutput(ctx, marshData)
		if updErr != nil {
			return nil, updErr
		}

		return []EditBlock{{State: updState, Output: data, StepName: stepName, StepID: stepID}}, nil

	case MonitoringTaskUpdateBlockRequestChangeTypeState:
		blockOutputs, stateErr := ae.DB.GetBlockOutputs(ctx, stepID.String(), stepName)
		if stateErr != nil {
			return nil, stateErr
		}

		funcOutputs := make(map[string]interface{})

		for i := range blockOutputs {
			output := blockOutputs[i]
			funcOutputs[output.Name] = output.Value
		}

		funcState := pipeline.ExecutableFunction{
			Mapping:       make(map[string]script.JSONSchemaPropertiesValue, 0),
			Constants:     make(map[string]interface{}, 0),
			RetryTimeouts: make([]int, 0),
		}

		unmErr := json.Unmarshal(marshData, &funcState)
		if unmErr != nil {
			return nil, unmErr
		}

		block := pipeline.ExecutableFunctionBlock{
			State: &funcState,
		}

		updOutput, updErr := block.UpdateOutputUsingState(ctx)
		if updErr != nil {
			return nil, updErr
		}

		for k, v := range updOutput {
			funcOutputs[k] = v
		}

		return []EditBlock{{State: data, Output: funcOutputs, StepName: stepName, StepID: stepID}}, nil
	}

	return res, nil
}

// nolint:dupl //duplicate is ok here
func (ae *Env) formEditBlock(ctx c.Context, stepID uuid.UUID, stepName string, data map[string]interface{},
	updateType MonitoringTaskUpdateBlockRequestChangeType,
) (res []EditBlock, err error) {
	marshData, marshErr := json.Marshal(data)
	if marshErr != nil {
		return nil, marshErr
	}

	switch updateType {
	case MonitoringTaskUpdateBlockRequestChangeTypeContext:
		return ae.editBlockContext(ctx, stepID, data)
	case MonitoringTaskUpdateBlockRequestChangeTypeOutput:
		blockState, stateErr := ae.DB.GetBlockState(ctx, stepID.String())
		if stateErr != nil {
			return nil, stateErr
		}

		formBlockState := pipeline.FormData{
			Executors:          make(map[string]struct{}, 0),
			InitialExecutors:   make(map[string]struct{}, 0),
			ApplicationBody:    make(map[string]interface{}, 0),
			Constants:          make(map[string]interface{}, 0),
			ChangesLog:         make([]pipeline.ChangesLogItem, 0),
			HiddenFields:       make([]string, 0),
			FormsAccessibility: make([]script.FormAccessibility, 0),
			Mapping:            make(map[string]script.JSONSchemaPropertiesValue, 0),
			AttachmentFields:   make([]string, 0),
			Keys:               make(map[string]string, 0),
		}

		unmErr := json.Unmarshal(blockState, &formBlockState)
		if unmErr != nil {
			return nil, unmErr
		}

		block := pipeline.GoFormBlock{State: &formBlockState}

		updState, updErr := block.UpdateStateUsingOutput(ctx, marshData)
		if updErr != nil {
			return nil, updErr
		}

		return []EditBlock{{State: updState, Output: data, StepName: stepName, StepID: stepID}}, nil

	case MonitoringTaskUpdateBlockRequestChangeTypeState:
		formBlockState := pipeline.FormData{
			Executors:          make(map[string]struct{}, 0),
			InitialExecutors:   make(map[string]struct{}, 0),
			ApplicationBody:    make(map[string]interface{}, 0),
			Constants:          make(map[string]interface{}, 0),
			ChangesLog:         make([]pipeline.ChangesLogItem, 0),
			HiddenFields:       make([]string, 0),
			FormsAccessibility: make([]script.FormAccessibility, 0),
			Mapping:            make(map[string]script.JSONSchemaPropertiesValue, 0),
			AttachmentFields:   make([]string, 0),
			Keys:               make(map[string]string, 0),
		}

		unmErr := json.Unmarshal(marshData, &formBlockState)
		if unmErr != nil {
			return nil, unmErr
		}

		block := pipeline.GoFormBlock{
			State:      &formBlockState,
			RunContext: &pipeline.BlockRunContext{Services: pipeline.RunContextServices{ServiceDesc: ae.ServiceDesc}},
		}

		updOutput, updErr := block.UpdateOutputUsingState(ctx)
		if updErr != nil {
			return nil, updErr
		}

		return []EditBlock{{State: data, Output: updOutput, StepName: stepName, StepID: stepID}}, nil
	}

	return res, nil
}

// nolint:dupl //duplicate is ok here
func (ae *Env) ifEditBlock(ctx c.Context, stepID uuid.UUID, stepName string, data map[string]interface{},
	updateType MonitoringTaskUpdateBlockRequestChangeType,
) (res []EditBlock, err error) {
	switch updateType {
	case MonitoringTaskUpdateBlockRequestChangeTypeContext:
		return ae.editBlockContext(ctx, stepID, data)
	case MonitoringTaskUpdateBlockRequestChangeTypeOutput:
		return []EditBlock{{State: map[string]interface{}{}, Output: data, StepName: stepName, StepID: stepID}}, nil
	case MonitoringTaskUpdateBlockRequestChangeTypeState:
		return []EditBlock{{State: data, Output: map[string]interface{}{}, StepName: stepName, StepID: stepID}}, nil
	}

	return res, nil
}

// nolint:dupl //duplicate is ok here
func (ae *Env) notificationEditBlock(ctx c.Context, stepID uuid.UUID, stepName string, data map[string]interface{},
	updateType MonitoringTaskUpdateBlockRequestChangeType,
) (res []EditBlock, err error) {
	switch updateType {
	case MonitoringTaskUpdateBlockRequestChangeTypeContext:
		return ae.editBlockContext(ctx, stepID, data)
	case MonitoringTaskUpdateBlockRequestChangeTypeOutput:
		blockState, stateErr := ae.DB.GetBlockState(ctx, stepID.String())
		if stateErr != nil {
			return nil, stateErr
		}

		notifState := make(map[string]interface{})

		unmErr := json.Unmarshal(blockState, &notifState)
		if unmErr != nil {
			return nil, unmErr
		}

		return []EditBlock{{State: notifState, Output: data, StepName: stepName, StepID: stepID}}, nil
	case MonitoringTaskUpdateBlockRequestChangeTypeState:
		return []EditBlock{{State: data, Output: map[string]interface{}{}, StepName: stepName, StepID: stepID}}, nil
	}

	return res, nil
}

// nolint:dupl //duplicate is ok here
func (ae *Env) sdEditBlock(ctx c.Context, stepID uuid.UUID, stepName string, data map[string]interface{},
	updateType MonitoringTaskUpdateBlockRequestChangeType,
) (res []EditBlock, err error) {
	marshData, marshErr := json.Marshal(data)
	if marshErr != nil {
		return nil, marshErr
	}

	switch updateType {
	case MonitoringTaskUpdateBlockRequestChangeTypeContext:
		return ae.editBlockContext(ctx, stepID, data)
	case MonitoringTaskUpdateBlockRequestChangeTypeOutput:
		blockState, stateErr := ae.DB.GetBlockState(ctx, stepID.String())
		if stateErr != nil {
			return nil, stateErr
		}

		sdState := pipeline.ApplicationData{
			ApplicationBody: make(map[string]interface{}, 0),
		}

		unmErr := json.Unmarshal(blockState, &sdState)
		if unmErr != nil {
			return nil, unmErr
		}

		block := pipeline.GoSdApplicationBlock{State: &sdState}

		updState, updErr := block.UpdateStateUsingOutput(ctx, marshData)
		if updErr != nil {
			return nil, updErr
		}

		return []EditBlock{{State: updState, Output: data, StepName: stepName, StepID: stepID}}, nil

	case MonitoringTaskUpdateBlockRequestChangeTypeState:
		sdState := pipeline.ApplicationData{
			ApplicationBody: make(map[string]interface{}, 0),
		}

		unmErr := json.Unmarshal(marshData, &sdState)
		if unmErr != nil {
			return nil, unmErr
		}

		block := pipeline.GoSdApplicationBlock{
			State:      &sdState,
			RunContext: &pipeline.BlockRunContext{Services: pipeline.RunContextServices{ServiceDesc: ae.ServiceDesc}},
		}

		updOutput, updErr := block.UpdateOutputUsingState(ctx)
		if updErr != nil {
			return nil, updErr
		}

		return []EditBlock{{State: data, Output: updOutput, StepName: stepName, StepID: stepID}}, nil
	}

	return res, nil
}

// nolint:dupl //duplicate is ok here
func (ae *Env) signEditBlock(ctx c.Context, stepID uuid.UUID, stepName string, data map[string]interface{},
	updateType MonitoringTaskUpdateBlockRequestChangeType,
) (res []EditBlock, err error) {
	marshData, marshErr := json.Marshal(data)
	if marshErr != nil {
		return nil, marshErr
	}

	switch updateType {
	case MonitoringTaskUpdateBlockRequestChangeTypeContext:
		return ae.editBlockContext(ctx, stepID, data)
	case MonitoringTaskUpdateBlockRequestChangeTypeOutput:
		blockState, stateErr := ae.DB.GetBlockState(ctx, stepID.String())
		if stateErr != nil {
			return nil, stateErr
		}

		signState := pipeline.SignData{
			Signers:             make(map[string]struct{}, 0),
			Attachments:         make([]entity.Attachment, 0),
			Signatures:          make([]pipeline.FileSignaturePair, 0),
			SignLog:             make([]pipeline.SignLogEntry, 0),
			FormsAccessibility:  make([]script.FormAccessibility, 0),
			AdditionalApprovers: make([]pipeline.AdditionalSignApprover, 0),
		}

		unmErr := json.Unmarshal(blockState, &signState)
		if unmErr != nil {
			return nil, unmErr
		}

		block := pipeline.GoSignBlock{State: &signState}

		updState, updErr := block.UpdateStateUsingOutput(ctx, marshData)
		if updErr != nil {
			return nil, updErr
		}

		return []EditBlock{{State: updState, Output: data, StepName: stepName, StepID: stepID}}, nil

	case MonitoringTaskUpdateBlockRequestChangeTypeState:
		signState := pipeline.SignData{
			Signers:             make(map[string]struct{}, 0),
			Attachments:         make([]entity.Attachment, 0),
			Signatures:          make([]pipeline.FileSignaturePair, 0),
			SignLog:             make([]pipeline.SignLogEntry, 0),
			FormsAccessibility:  make([]script.FormAccessibility, 0),
			AdditionalApprovers: make([]pipeline.AdditionalSignApprover, 0),
		}

		unmErr := json.Unmarshal(marshData, &signState)
		if unmErr != nil {
			return nil, unmErr
		}

		block := pipeline.GoSignBlock{
			State:      &signState,
			RunContext: &pipeline.BlockRunContext{Services: pipeline.RunContextServices{ServiceDesc: ae.ServiceDesc}},
		}

		updOutput, updErr := block.UpdateOutputUsingState(ctx)
		if updErr != nil {
			return nil, updErr
		}

		return []EditBlock{{State: data, Output: updOutput, StepName: stepName, StepID: stepID}}, nil
	}

	return res, nil
}

// nolint:dupl //duplicate is ok here
func (ae *Env) timerEditBlock(ctx c.Context, stepID uuid.UUID, stepName string, data map[string]interface{},
	updateType MonitoringTaskUpdateBlockRequestChangeType,
) (res []EditBlock, err error) {
	switch updateType {
	case MonitoringTaskUpdateBlockRequestChangeTypeContext:
		return ae.editBlockContext(ctx, stepID, data)
	case MonitoringTaskUpdateBlockRequestChangeTypeOutput:
		blockState, stateErr := ae.DB.GetBlockState(ctx, stepID.String())
		if stateErr != nil {
			return nil, stateErr
		}

		timerState := make(map[string]interface{})

		unmErr := json.Unmarshal(blockState, &timerState)
		if unmErr != nil {
			return nil, unmErr
		}

		return []EditBlock{{State: timerState, Output: data, StepName: stepName, StepID: stepID}}, nil

	case MonitoringTaskUpdateBlockRequestChangeTypeState:
		return []EditBlock{{State: data, Output: map[string]interface{}{}, StepName: stepName, StepID: stepID}}, nil
	}

	return res, nil
}

func (ae *Env) editBlockContext(ctx c.Context, stepID uuid.UUID, data map[string]interface{}) (res []EditBlock, err error) {
	contextParams := map[string]map[string]interface{}{}

	for key, val := range data {
		splitCtxParam := strings.Split(key, ".")
		if len(splitCtxParam) < 2 {
			continue
		}

		if _, ok := contextParams[splitCtxParam[0]]; ok {
			contextParams[splitCtxParam[0]][splitCtxParam[1]] = val
		} else {
			contextParams[splitCtxParam[0]] = map[string]interface{}{splitCtxParam[1]: val}
		}
	}

	taskStep, stepErr := ae.DB.GetTaskStepByID(ctx, stepID)
	if stepErr != nil {
		return nil, stepErr
	}

	for paramKey, paramVal := range contextParams {
		dbStep, dbErr := ae.DB.GetTaskStepByNameForCtxEditing(ctx, taskStep.WorkID, paramKey, taskStep.Time)
		if dbErr != nil {
			return nil, dbErr
		}

		blockRes, contextErr := ae.editGoBlock(ctx, &editGoBlockDTO{
			stepID:     dbStep.ID,
			stepType:   dbStep.Type,
			stepName:   paramKey,
			data:       paramVal,
			updateType: MonitoringTaskUpdateBlockRequestChangeTypeOutput,
		})
		if contextErr != nil {
			return nil, contextErr
		}

		res = append(res, blockRes...)
	}

	return res, nil
}
