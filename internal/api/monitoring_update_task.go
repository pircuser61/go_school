package api

import (
	c "context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"github.com/google/uuid"
	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
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

	data := convertReqEditData(req.ChangeData.AdditionalProperties)

	blockUUID, parseIDErr := uuid.Parse(blockId)
	if parseIDErr != nil {
		errorHandler.handleError(UUIDParsingError, parseIDErr)
		ae.rollbackTransaction(ctx, txStorage, fn)

		return
	}

	dbStep, getStepErr := ae.DB.GetTaskStepByID(ctx, blockUUID)
	if getStepErr != nil {
		errorHandler.handleError(GetTaskStepError, getStepErr)
		ae.rollbackTransaction(ctx, txStorage, fn)

		return
	}

	log = log.WithField("stepName", dbStep.Name).
		WithField("workID", dbStep.WorkID).
		WithField("workNumber", dbStep.WorkNumber)
	ctx = logger.WithLogger(ctx, log)

	editBlockData, editErr := ae.editGoBlock(ctx, blockUUID, dbStep.Type, dbStep.Name, data, req.ChangeType)
	if editErr != nil {
		errorHandler.handleError(EditMonitoringBlockError, editErr)
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
		Steps:      []uuid.UUID{blockUUID},
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

func convertReqEditData(in map[string]MonitoringEditBlockData) (res map[string]interface{}) {
	res = map[string]interface{}{}
	for key, val := range in {
		res[key] = val.Value
	}

	return res
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
		savePrevErr := txStorage.SaveNodePreviousContent(ctx, data[i].StepID.String(), eventID)
		if savePrevErr != nil {
			return SaveNodePrevContentError
		}
	}

	for i := range data {
		saveErr := txStorage.UpdateNodeContent(ctx, data[i].StepID.String(), workID, data[i].StepName,
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

func (ae *Env) editGoBlock(ctx c.Context, stepID uuid.UUID, stepType, stepName string, data map[string]interface{},
	updateType MonitoringTaskUpdateBlockRequestChangeType,
) (res []EditBlock, err error) {
	switch stepType {
	case pipeline.BlockGoApproverID:
		res, err = ae.approverEditBlock(ctx, stepID, stepName, data, updateType)
	case pipeline.BlockGoExecutionID:
		res, err = ae.executorEditBlock(ctx, stepID, stepName, data, updateType)
	case pipeline.BlockGoStartID:
		res, err = ae.startEditBlock(ctx, stepID, stepName, data, updateType)
	case pipeline.BlockGoEndID:
		res, err = ae.endEditBlock(ctx, stepID, stepName, data, updateType)
	case pipeline.BlockGoBeginParallelTaskID:
		res, err = ae.startParallelEditBlock(ctx, stepID, stepName, data, updateType)
	case pipeline.BlockWaitForAllInputsID:
		res, err = ae.endParallelEditBlock(ctx, stepID, stepName, data, updateType)
	case pipeline.BlockExecutableFunctionID:
		res, err = ae.functionEditBlock(ctx, stepID, stepName, data, updateType)
	case pipeline.BlockGoFormID:
		res, err = ae.formEditBlock(ctx, stepID, stepName, data, updateType)
	case pipeline.BlockGoIfID:
		res, err = ae.ifEditBlock(ctx, stepID, stepName, data, updateType)
	case pipeline.BlockGoNotificationID:
		res, err = ae.notificationEditBlock(ctx, stepID, stepName, data, updateType)
	case pipeline.BlockGoSdApplicationID:
		res, err = ae.sdEditBlock(ctx, stepID, stepName, data, updateType)
	case pipeline.BlockGoSignID:
		res, err = ae.signEditBlock(ctx, stepID, stepName, data, updateType)
	case pipeline.BlockTimerID:
		res, err = ae.timerEditBlock(ctx, stepID, stepName, data, updateType)
	default:
		err = fmt.Errorf("unknown block type")
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

		approverState := pipeline.ApproverData{}

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
		approverState := pipeline.ApproverData{}

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

		execState := pipeline.ExecutionData{}

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
		execState := pipeline.ExecutionData{}

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

		funcState := pipeline.ExecutableFunction{}

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

		funcState := pipeline.ExecutableFunction{}

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

		formBlockState := pipeline.FormData{}

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
		formBlockState := pipeline.FormData{}

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

		sdState := pipeline.ApplicationData{}

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
		sdState := pipeline.ApplicationData{}

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

		signState := pipeline.SignData{}

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
		signState := pipeline.SignData{}

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
		splitedCtxParam := strings.Split(key, ".")
		if len(splitedCtxParam) < 2 {
			continue
		}

		if _, ok := contextParams[splitedCtxParam[0]]; ok {
			contextParams[splitedCtxParam[0]][splitedCtxParam[1]] = val
		} else {
			contextParams[splitedCtxParam[0]] = map[string]interface{}{splitedCtxParam[1]: val}
		}
	}

	taskStep, stepErr := ae.DB.GetTaskStepByID(ctx, stepID)
	if stepErr != nil {
		return nil, stepErr
	}

	for paramKey, paramVal := range contextParams {
		innerStep, inerStepErr := ae.DB.GetTaskStepByNameForCtxEditing(ctx, taskStep.WorkID, paramKey, taskStep.Time)
		if inerStepErr != nil {
			return nil, inerStepErr
		}

		blockRes, contextErr := ae.editGoBlock(ctx,
			innerStep.ID, innerStep.Type, paramKey, paramVal, MonitoringTaskUpdateBlockRequestChangeTypeOutput)
		if contextErr != nil {
			return nil, contextErr
		}

		res = append(res, blockRes...)
	}

	return res, nil
}
