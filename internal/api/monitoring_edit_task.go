package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"github.com/google/uuid"
	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/user"
)

// nolint:revive
func (ae *Env) EditTaskBlockData(w http.ResponseWriter, r *http.Request, blockId string) {
	ctx, span := trace.StartSpan(r.Context(), "monitoring_edit_task_block_data")
	defer span.End()

	log := logger.GetLogger(ctx)
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

	req := &MonitoringTaskEditBlockRequest{}

	err = json.Unmarshal(b, req)
	if err != nil {
		errorHandler.handleError(MonitoringEditBlockParseError, err)

		return
	}

	data := convertReqEditData(req.ChangeData.AdditionalProperties)

	blockUUID, parseIdErr := uuid.Parse(blockId)
	if parseIdErr != nil {
		errorHandler.handleError(UUIDParsingError, err)

		return
	}

	dbStep, getStepErr := ae.DB.GetTaskStepByID(ctx, blockUUID)
	if getStepErr != nil {
		errorHandler.handleError(GetTaskStepError, getStepErr)

		return
	}

	editBlockData, editErr := ae.editGoBlock(ctx, blockUUID, dbStep.Type, dbStep.Name, data, req.ChangeType)
	if editErr != nil {
		errorHandler.handleError(EditMonitoringBlockError, err)
	}

	ui, err := user.GetUserInfoFromCtx(ctx)
	if err != nil {
		errorHandler.handleError(NoUserInContextError, err)

		return
	}

	eventData := struct {
		Data    map[string]interface{}
		BlockId uuid.UUID
	}{Data: data, BlockId: blockUUID}

	jsonParams := json.RawMessage{}
	jsonParams, err = json.Marshal(eventData)
	if err != nil {
		errorHandler.handleError(MarshalEventParamsError, err)
	}

	eventId, err := txStorage.CreateTaskEvent(ctx, &entity.CreateTaskEvent{
		WorkID:    dbStep.WorkID.String(),
		Author:    ui.Username,
		EventType: string(MonitoringTaskActionRequestActionStart),
		Params:    jsonParams,
	})
	if err != nil {
		errorHandler.handleError(CreateTaskEventError, err)
	}

	eventUUID, parseIdErr := uuid.Parse(eventId)
	if parseIdErr != nil {
		errorHandler.handleError(UUIDParsingError, err)

		return
	}

	for i := range editBlockData {
		savePrevErr := txStorage.SaveNodePreviousContent(ctx, editBlockData[i].StepId, eventUUID)
		if savePrevErr != nil {
			errorHandler.handleError(SaveNodePrevContentError, savePrevErr)
		}
	}

	for i := range editBlockData {
		saveErr := txStorage.UpdateNodeContent(ctx, editBlockData[i].StepId, dbStep.WorkID, editBlockData[i].StepName,
			editBlockData[i].State, editBlockData[i].Output)
		if saveErr != nil {
			errorHandler.handleError(SaveUpdatedBlockData, saveErr)
		}
	}

	return
}

func convertReqEditData(reqData map[string]MonitoringEditBlockData) (res map[string]interface{}) {
	convertedData := map[string]interface{}{}
	for key, val := range reqData {
		convertedData[key] = val.Value
	}
	return convertedData
}

type EditBlock struct {
	StepId        uuid.UUID
	StepName      string
	State, Output map[string]interface{}
}

// метод должен возвращть массив структур вида stepId uuid,степнейм стринг  state, context map[str]interface{}
// в момент обраб
// все методы *editblock должны принимать степ айди, степнейм, дата, едиттайп, а возвращать массив структур stepId uuid,степнейм,  state, context map[str]interface{}
func (ae *Env) editGoBlock(ctx context.Context, stepId uuid.UUID, stepType, stepName string, data map[string]interface{},
	editType MonitoringTaskEditBlockRequestChangeType,
) (res []EditBlock, err error) {
	switch stepType {
	case pipeline.BlockGoApproverID:
		res, err = ae.approverEditBlock(ctx, stepId, stepType, stepName, data, editType)
	case pipeline.BlockGoExecutionID:
		res, err = ae.executorEditBlock(ctx, stepId, stepType, stepName, data, editType)
	case pipeline.BlockGoStartID:
		res, err = ae.startEditBlock(ctx, stepId, stepType, stepName, data, editType)
	case pipeline.BlockGoEndID:
		res, err = ae.endEditBlock(ctx, stepId, stepType, stepName, data, editType)
	case pipeline.BlockGoBeginParallelTaskID:
		res, err = ae.startParallelEditBlock(ctx, stepId, stepType, stepName, data, editType)
	case pipeline.BlockWaitForAllInputsID:
		res, err = ae.endParallelEditBlock(ctx, stepId, stepType, stepName, data, editType)
	case pipeline.BlockExecutableFunctionID:
		res, err = ae.functionEditBlock(ctx, stepId, stepType, stepName, data, editType)
	case pipeline.BlockGoFormID:
		res, err = ae.formEditBlock(ctx, stepId, stepType, stepName, data, editType)
	case pipeline.BlockGoIfID:
		res, err = ae.ifEditBlock(ctx, stepId, stepType, stepName, data, editType)
	case pipeline.BlockGoNotificationID:
		res, err = ae.notificationEditBlock(ctx, stepId, stepType, stepName, data, editType)
	case pipeline.BlockGoSdApplicationID:
		res, err = ae.sdEditBlock(ctx, stepId, stepType, stepName, data, editType)
	case pipeline.BlockGoSignID:
		res, err = ae.signEditBlock(ctx, stepId, stepType, stepName, data, editType)
	case pipeline.BlockTimerID:
		res, err = ae.timerEditBlock(ctx, stepId, stepType, stepName, data, editType)
	default:
		err = fmt.Errorf("unknown block type")
	}

	return res, err
}

func (ae *Env) approverEditBlock(ctx context.Context, stepId uuid.UUID, stepType, stepName string, data map[string]interface{},
	editType MonitoringTaskEditBlockRequestChangeType,
) (res []EditBlock, err error) {
	marshData, marshErr := json.Marshal(data)
	if marshErr != nil {
		return nil, marshErr
	}

	switch editType {
	case MonitoringTaskEditBlockRequestChangeTypeContext:
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

		taskStep, stepErr := ae.DB.GetTaskStepByID(ctx, stepId)
		if stepErr != nil {
			return nil, stepErr
		}

		for paramKey, paramVal := range contextParams {
			innerStep, inerStepErr := ae.DB.GetTaskStepByNameForCtxEditing(ctx, taskStep.WorkID, paramKey, taskStep.Time)
			if inerStepErr != nil {
				return nil, inerStepErr
			}

			blockRes, contextErr := ae.editGoBlock(ctx, innerStep.ID, innerStep.Type, paramKey, paramVal, MonitoringTaskEditBlockRequestChangeTypeOutput)
			if contextErr != nil {
				return nil, contextErr
			}

			res = append(res, blockRes...)
		}

	case MonitoringTaskEditBlockRequestChangeTypeInput:
		appParams := script.ApproverParams{}

		unmErr := json.Unmarshal(marshData, &appParams)
		if unmErr != nil {
			return nil, unmErr
		}

		validateErr := appParams.Validate()
		if validateErr != nil {
			return nil, validateErr
		}

	case MonitoringTaskEditBlockRequestChangeTypeOutput:
		blockState, stateErr := ae.DB.GetBlockState(ctx, stepId.String())
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

		return []EditBlock{{State: updState, Output: data, StepName: stepName, StepId: stepId}}, nil

	case MonitoringTaskEditBlockRequestChangeTypeState:
		approverState := pipeline.ApproverData{}

		unmErr := json.Unmarshal(marshData, &approverState)
		if unmErr != nil {
			return nil, unmErr
		}

		block := pipeline.GoApproverBlock{State: &approverState}

		updOutput, updErr := block.UpdateOutputUsingState(ctx)
		if updErr != nil {
			return nil, updErr
		}

		return []EditBlock{{State: data, Output: updOutput, StepName: stepName, StepId: stepId}}, nil
	}

	return res, nil
}

func (ae *Env) executorEditBlock(ctx context.Context, stepId uuid.UUID, stepType, stepName string, data map[string]interface{},
	editType MonitoringTaskEditBlockRequestChangeType,
) (res []EditBlock, err error) {
	marshData, marshErr := json.Marshal(data)
	if marshErr != nil {
		return nil, marshErr
	}

	switch editType {
	case MonitoringTaskEditBlockRequestChangeTypeContext:
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

		taskStep, stepErr := ae.DB.GetTaskStepByID(ctx, stepId)
		if stepErr != nil {
			return nil, stepErr
		}

		for paramKey, paramVal := range contextParams {
			innerStep, inerStepErr := ae.DB.GetTaskStepByNameForCtxEditing(ctx, taskStep.WorkID, paramKey, taskStep.Time)
			if inerStepErr != nil {
				return nil, inerStepErr
			}

			blockRes, contextErr := ae.editGoBlock(ctx, innerStep.ID, innerStep.Type, paramKey, paramVal, MonitoringTaskEditBlockRequestChangeTypeOutput)
			if contextErr != nil {
				return nil, contextErr
			}

			res = append(res, blockRes...)
		}

	case MonitoringTaskEditBlockRequestChangeTypeInput:
		execParams := script.ExecutionParams{}

		unmErr := json.Unmarshal(marshData, &execParams)
		if unmErr != nil {
			return nil, unmErr
		}

		validateErr := execParams.Validate()
		if validateErr != nil {
			return nil, validateErr
		}

	case MonitoringTaskEditBlockRequestChangeTypeOutput:
		blockState, stateErr := ae.DB.GetBlockState(ctx, stepId.String())
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

		return []EditBlock{{State: updState, Output: data, StepName: stepName, StepId: stepId}}, nil

	case MonitoringTaskEditBlockRequestChangeTypeState:
		execState := pipeline.ExecutionData{}

		unmErr := json.Unmarshal(marshData, &execState)
		if unmErr != nil {
			return nil, unmErr
		}

		block := pipeline.GoExecutionBlock{State: &execState}

		updOutput, updErr := block.UpdateOutputUsingState(ctx)
		if updErr != nil {
			return nil, updErr
		}

		return []EditBlock{{State: data, Output: updOutput, StepName: stepName, StepId: stepId}}, nil
	}

	return res, nil
}

func (ae *Env) startEditBlock(ctx context.Context, stepId uuid.UUID, stepType, stepName string, data map[string]interface{},
	editType MonitoringTaskEditBlockRequestChangeType,
) (res []EditBlock, err error) {
	switch editType {
	case MonitoringTaskEditBlockRequestChangeTypeContext:

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

		taskStep, stepErr := ae.DB.GetTaskStepByID(ctx, stepId)
		if stepErr != nil {
			return nil, stepErr
		}

		for paramKey, paramVal := range contextParams {
			innerStep, inerStepErr := ae.DB.GetTaskStepByNameForCtxEditing(ctx, taskStep.WorkID, paramKey, taskStep.Time)
			if inerStepErr != nil {
				return nil, inerStepErr
			}

			blockRes, contextErr := ae.editGoBlock(ctx, innerStep.ID, innerStep.Type, paramKey, paramVal, MonitoringTaskEditBlockRequestChangeTypeOutput)
			if contextErr != nil {
				return nil, contextErr
			}

			res = append(res, blockRes...)
		}

	case MonitoringTaskEditBlockRequestChangeTypeInput:
	case MonitoringTaskEditBlockRequestChangeTypeOutput:
		return []EditBlock{{State: map[string]interface{}{}, Output: data, StepName: stepName, StepId: stepId}}, nil

	case MonitoringTaskEditBlockRequestChangeTypeState:
		return []EditBlock{{State: data, Output: map[string]interface{}{}, StepName: stepName, StepId: stepId}}, nil

	}

	return res, nil
}

func (ae *Env) endEditBlock(ctx context.Context, stepId uuid.UUID, stepType, stepName string, data map[string]interface{},
	editType MonitoringTaskEditBlockRequestChangeType,
) (res []EditBlock, err error) {
	switch editType {
	case MonitoringTaskEditBlockRequestChangeTypeContext:
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

		taskStep, stepErr := ae.DB.GetTaskStepByID(ctx, stepId)
		if stepErr != nil {
			return nil, stepErr
		}

		for paramKey, paramVal := range contextParams {
			innerStep, inerStepErr := ae.DB.GetTaskStepByNameForCtxEditing(ctx, taskStep.WorkID, paramKey, taskStep.Time)
			if inerStepErr != nil {
				return nil, inerStepErr
			}

			blockRes, contextErr := ae.editGoBlock(ctx, innerStep.ID, innerStep.Type, paramKey, paramVal, MonitoringTaskEditBlockRequestChangeTypeOutput)
			if contextErr != nil {
				return nil, contextErr
			}

			res = append(res, blockRes...)
		}

	case MonitoringTaskEditBlockRequestChangeTypeInput:
	case MonitoringTaskEditBlockRequestChangeTypeOutput:
		return []EditBlock{{State: map[string]interface{}{}, Output: data, StepName: stepName, StepId: stepId}}, nil

	case MonitoringTaskEditBlockRequestChangeTypeState:
		return []EditBlock{{State: data, Output: map[string]interface{}{}, StepName: stepName, StepId: stepId}}, nil

	}

	return res, nil
}

func (ae *Env) startParallelEditBlock(ctx context.Context, stepId uuid.UUID, stepType, stepName string, data map[string]interface{},
	editType MonitoringTaskEditBlockRequestChangeType,
) (res []EditBlock, err error) {
	switch editType {
	case MonitoringTaskEditBlockRequestChangeTypeContext:
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

		taskStep, stepErr := ae.DB.GetTaskStepByID(ctx, stepId)
		if stepErr != nil {
			return nil, stepErr
		}

		for paramKey, paramVal := range contextParams {
			innerStep, inerStepErr := ae.DB.GetTaskStepByNameForCtxEditing(ctx, taskStep.WorkID, paramKey, taskStep.Time)
			if inerStepErr != nil {
				return nil, inerStepErr
			}

			blockRes, contextErr := ae.editGoBlock(ctx, innerStep.ID, innerStep.Type, paramKey, paramVal, MonitoringTaskEditBlockRequestChangeTypeOutput)
			if contextErr != nil {
				return nil, contextErr
			}

			res = append(res, blockRes...)
		}

	case MonitoringTaskEditBlockRequestChangeTypeInput:
	case MonitoringTaskEditBlockRequestChangeTypeOutput:
		return []EditBlock{{State: map[string]interface{}{}, Output: data, StepName: stepName, StepId: stepId}}, nil

	case MonitoringTaskEditBlockRequestChangeTypeState:
		return []EditBlock{{State: data, Output: map[string]interface{}{}, StepName: stepName, StepId: stepId}}, nil

	}

	return res, nil
}

func (ae *Env) endParallelEditBlock(ctx context.Context, stepId uuid.UUID, stepType, stepName string, data map[string]interface{},
	editType MonitoringTaskEditBlockRequestChangeType,
) (res []EditBlock, err error) {
	switch editType {
	case MonitoringTaskEditBlockRequestChangeTypeContext:
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

		taskStep, stepErr := ae.DB.GetTaskStepByID(ctx, stepId)
		if stepErr != nil {
			return nil, stepErr
		}

		for paramKey, paramVal := range contextParams {
			innerStep, inerStepErr := ae.DB.GetTaskStepByNameForCtxEditing(ctx, taskStep.WorkID, paramKey, taskStep.Time)
			if inerStepErr != nil {
				return nil, inerStepErr
			}

			blockRes, contextErr := ae.editGoBlock(ctx, innerStep.ID, innerStep.Type, paramKey, paramVal, MonitoringTaskEditBlockRequestChangeTypeOutput)
			if contextErr != nil {
				return nil, contextErr
			}

			res = append(res, blockRes...)
		}

	case MonitoringTaskEditBlockRequestChangeTypeInput:
	case MonitoringTaskEditBlockRequestChangeTypeOutput:
		return []EditBlock{{State: map[string]interface{}{}, Output: data, StepName: stepName, StepId: stepId}}, nil

	case MonitoringTaskEditBlockRequestChangeTypeState:
		return []EditBlock{{State: data, Output: map[string]interface{}{}, StepName: stepName, StepId: stepId}}, nil

	}

	return res, nil
}

func (ae *Env) functionEditBlock(ctx context.Context, stepId uuid.UUID, stepType, stepName string, data map[string]interface{},
	editType MonitoringTaskEditBlockRequestChangeType,
) (res []EditBlock, err error) {
	marshData, marshErr := json.Marshal(data)
	if marshErr != nil {
		return nil, marshErr
	}

	switch editType {
	case MonitoringTaskEditBlockRequestChangeTypeContext:
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

		taskStep, stepErr := ae.DB.GetTaskStepByID(ctx, stepId)
		if stepErr != nil {
			return nil, stepErr
		}

		for paramKey, paramVal := range contextParams {
			innerStep, inerStepErr := ae.DB.GetTaskStepByNameForCtxEditing(ctx, taskStep.WorkID, paramKey, taskStep.Time)
			if inerStepErr != nil {
				return nil, inerStepErr
			}

			blockRes, contextErr := ae.editGoBlock(ctx, innerStep.ID, innerStep.Type, paramKey, paramVal, MonitoringTaskEditBlockRequestChangeTypeOutput)
			if contextErr != nil {
				return nil, contextErr
			}

			res = append(res, blockRes...)
		}

	case MonitoringTaskEditBlockRequestChangeTypeInput:
		funcParams := script.FunctionParams{}

		unmErr := json.Unmarshal(marshData, &funcParams)
		if unmErr != nil {
			return nil, unmErr
		}
		// TODO no validate method
	case MonitoringTaskEditBlockRequestChangeTypeOutput:
		return []EditBlock{{State: map[string]interface{}{}, Output: data, StepName: stepName, StepId: stepId}}, nil

	case MonitoringTaskEditBlockRequestChangeTypeState:
		return []EditBlock{{State: data, Output: map[string]interface{}{}, StepName: stepName, StepId: stepId}}, nil

	}

	return res, nil
}

func (ae *Env) formEditBlock(ctx context.Context, stepId uuid.UUID, stepType, stepName string, data map[string]interface{},
	editType MonitoringTaskEditBlockRequestChangeType,
) (res []EditBlock, err error) {
	marshData, marshErr := json.Marshal(data)
	if marshErr != nil {
		return nil, marshErr
	}

	switch editType {
	case MonitoringTaskEditBlockRequestChangeTypeContext:
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

		taskStep, stepErr := ae.DB.GetTaskStepByID(ctx, stepId)
		if stepErr != nil {
			return nil, stepErr
		}

		for paramKey, paramVal := range contextParams {
			innerStep, inerStepErr := ae.DB.GetTaskStepByNameForCtxEditing(ctx, taskStep.WorkID, paramKey, taskStep.Time)
			if inerStepErr != nil {
				return nil, inerStepErr
			}

			blockRes, contextErr := ae.editGoBlock(ctx, innerStep.ID, innerStep.Type, paramKey, paramVal, MonitoringTaskEditBlockRequestChangeTypeOutput)
			if contextErr != nil {
				return nil, contextErr
			}

			res = append(res, blockRes...)
		}

	case MonitoringTaskEditBlockRequestChangeTypeInput:
		formParams := script.FormParams{}

		unmErr := json.Unmarshal(marshData, &formParams)
		if unmErr != nil {
			return nil, unmErr
		}

		validateErr := formParams.Validate()
		if validateErr != nil {
			return nil, validateErr
		}

	case MonitoringTaskEditBlockRequestChangeTypeOutput:
		blockState, stateErr := ae.DB.GetBlockState(ctx, stepId.String())
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

		return []EditBlock{{State: updState, Output: data, StepName: stepName, StepId: stepId}}, nil

	case MonitoringTaskEditBlockRequestChangeTypeState:
		formBlockState := pipeline.FormData{}

		unmErr := json.Unmarshal(marshData, &formBlockState)
		if unmErr != nil {
			return nil, unmErr
		}

		block := pipeline.GoFormBlock{State: &formBlockState}

		updOutput, updErr := block.UpdateOutputUsingState(ctx)
		if updErr != nil {
			return nil, updErr
		}

		return []EditBlock{{State: data, Output: updOutput, StepName: stepName, StepId: stepId}}, nil
	}

	return res, nil
}

func (ae *Env) ifEditBlock(ctx context.Context, stepId uuid.UUID, stepType, stepName string, data map[string]interface{},
	editType MonitoringTaskEditBlockRequestChangeType,
) (res []EditBlock, err error) {
	switch editType {
	case MonitoringTaskEditBlockRequestChangeTypeContext:
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

		taskStep, stepErr := ae.DB.GetTaskStepByID(ctx, stepId)
		if stepErr != nil {
			return nil, stepErr
		}

		for paramKey, paramVal := range contextParams {
			innerStep, inerStepErr := ae.DB.GetTaskStepByNameForCtxEditing(ctx, taskStep.WorkID, paramKey, taskStep.Time)
			if inerStepErr != nil {
				return nil, inerStepErr
			}

			blockRes, contextErr := ae.editGoBlock(ctx, innerStep.ID, innerStep.Type, paramKey, paramVal, MonitoringTaskEditBlockRequestChangeTypeOutput)
			if contextErr != nil {
				return nil, contextErr
			}

			res = append(res, blockRes...)
		}

	case MonitoringTaskEditBlockRequestChangeTypeInput:
	case MonitoringTaskEditBlockRequestChangeTypeOutput:
		return []EditBlock{{State: map[string]interface{}{}, Output: data, StepName: stepName, StepId: stepId}}, nil

	case MonitoringTaskEditBlockRequestChangeTypeState:
		return []EditBlock{{State: data, Output: map[string]interface{}{}, StepName: stepName, StepId: stepId}}, nil

	}

	return res, nil
}

func (ae *Env) notificationEditBlock(ctx context.Context, stepId uuid.UUID, stepType, stepName string, data map[string]interface{},
	editType MonitoringTaskEditBlockRequestChangeType,
) (res []EditBlock, err error) {
	marshData, marshErr := json.Marshal(data)
	if marshErr != nil {
		return nil, marshErr
	}

	switch editType {
	case MonitoringTaskEditBlockRequestChangeTypeContext:
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

		taskStep, stepErr := ae.DB.GetTaskStepByID(ctx, stepId)
		if stepErr != nil {
			return nil, stepErr
		}

		for paramKey, paramVal := range contextParams {
			innerStep, inerStepErr := ae.DB.GetTaskStepByNameForCtxEditing(ctx, taskStep.WorkID, paramKey, taskStep.Time)
			if inerStepErr != nil {
				return nil, inerStepErr
			}

			blockRes, contextErr := ae.editGoBlock(ctx, innerStep.ID, innerStep.Type, paramKey, paramVal, MonitoringTaskEditBlockRequestChangeTypeOutput)
			if contextErr != nil {
				return nil, contextErr
			}

			res = append(res, blockRes...)
		}

	case MonitoringTaskEditBlockRequestChangeTypeInput:
		notifParams := script.NotificationParams{}

		unmErr := json.Unmarshal(marshData, &notifParams)
		if unmErr != nil {
			return nil, unmErr
		}

		validateErr := notifParams.Validate()
		if validateErr != nil {
			return nil, validateErr
		}

	case MonitoringTaskEditBlockRequestChangeTypeOutput:
		return []EditBlock{{State: map[string]interface{}{}, Output: data, StepName: stepName, StepId: stepId}}, nil

	case MonitoringTaskEditBlockRequestChangeTypeState:
		return []EditBlock{{State: data, Output: map[string]interface{}{}, StepName: stepName, StepId: stepId}}, nil

	}

	return res, nil
}

func (ae *Env) sdEditBlock(ctx context.Context, stepId uuid.UUID, stepType, stepName string, data map[string]interface{},
	editType MonitoringTaskEditBlockRequestChangeType,
) (res []EditBlock, err error) {
	marshData, marshErr := json.Marshal(data)
	if marshErr != nil {
		return nil, marshErr
	}

	switch editType {
	case MonitoringTaskEditBlockRequestChangeTypeContext:
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

		taskStep, stepErr := ae.DB.GetTaskStepByID(ctx, stepId)
		if stepErr != nil {
			return nil, stepErr
		}

		for paramKey, paramVal := range contextParams {
			innerStep, inerStepErr := ae.DB.GetTaskStepByNameForCtxEditing(ctx, taskStep.WorkID, paramKey, taskStep.Time)
			if inerStepErr != nil {
				return nil, inerStepErr
			}

			blockRes, contextErr := ae.editGoBlock(ctx, innerStep.ID, innerStep.Type, paramKey, paramVal, MonitoringTaskEditBlockRequestChangeTypeOutput)
			if contextErr != nil {
				return nil, contextErr
			}

			res = append(res, blockRes...)
		}

	case MonitoringTaskEditBlockRequestChangeTypeInput:
		sdParams := script.SdApplicationParams{}

		unmErr := json.Unmarshal(marshData, &sdParams)
		if unmErr != nil {
			return nil, unmErr
		}

		validateErr := sdParams.Validate()
		if validateErr != nil {
			return nil, validateErr
		}

	case MonitoringTaskEditBlockRequestChangeTypeOutput:
		blockState, stateErr := ae.DB.GetBlockState(ctx, stepId.String())
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

		return []EditBlock{{State: updState, Output: data, StepName: stepName, StepId: stepId}}, nil

	case MonitoringTaskEditBlockRequestChangeTypeState:
		sdState := pipeline.ApplicationData{}

		unmErr := json.Unmarshal(marshData, &sdState)
		if unmErr != nil {
			return nil, unmErr
		}

		block := pipeline.GoSdApplicationBlock{State: &sdState}

		updOutput, updErr := block.UpdateOutputUsingState(ctx)
		if updErr != nil {
			return nil, updErr
		}

		return []EditBlock{{State: data, Output: updOutput, StepName: stepName, StepId: stepId}}, nil
	}

	return res, nil
}

func (ae *Env) signEditBlock(ctx context.Context, stepId uuid.UUID, stepType, stepName string, data map[string]interface{},
	editType MonitoringTaskEditBlockRequestChangeType,
) (res []EditBlock, err error) {
	marshData, marshErr := json.Marshal(data)
	if marshErr != nil {
		return nil, marshErr
	}
	taskStep, stepErr := ae.DB.GetTaskStepByID(ctx, stepId)
	if stepErr != nil {
		return nil, stepErr
	}

	switch editType {
	case MonitoringTaskEditBlockRequestChangeTypeContext:
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

		for paramKey, paramVal := range contextParams {
			innerStep, inerStepErr := ae.DB.GetTaskStepByNameForCtxEditing(ctx, taskStep.WorkID, paramKey, taskStep.Time)
			if inerStepErr != nil {
				return nil, inerStepErr
			}

			blockRes, contextErr := ae.editGoBlock(ctx, innerStep.ID, innerStep.Type, paramKey, paramVal, MonitoringTaskEditBlockRequestChangeTypeOutput)
			if contextErr != nil {
				return nil, contextErr
			}

			res = append(res, blockRes...)
		}
	case MonitoringTaskEditBlockRequestChangeTypeInput:
		signParams := script.SignParams{}

		unmErr := json.Unmarshal(marshData, &signParams)
		if unmErr != nil {
			return nil, unmErr
		}

		validateErr := signParams.Validate()
		if validateErr != nil {
			return nil, validateErr
		}

	case MonitoringTaskEditBlockRequestChangeTypeOutput:
		blockState, stateErr := ae.DB.GetBlockState(ctx, stepId.String())
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

		return []EditBlock{{State: updState, Output: data, StepName: stepName, StepId: stepId}}, nil

	case MonitoringTaskEditBlockRequestChangeTypeState:
		signState := pipeline.SignData{}

		unmErr := json.Unmarshal(marshData, &signState)
		if unmErr != nil {
			return nil, unmErr
		}

		block := pipeline.GoSignBlock{State: &signState}

		updOutput, updErr := block.UpdateOutputUsingState(ctx)
		if updErr != nil {
			return nil, updErr
		}

		return []EditBlock{{State: data, Output: updOutput, StepName: stepName, StepId: stepId}}, nil

	}

	return res, nil
}

func (ae *Env) timerEditBlock(ctx context.Context, stepId uuid.UUID, stepType, stepName string, data map[string]interface{},
	editType MonitoringTaskEditBlockRequestChangeType,
) (res []EditBlock, err error) {
	switch editType {
	case MonitoringTaskEditBlockRequestChangeTypeContext:
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

		taskStep, stepErr := ae.DB.GetTaskStepByID(ctx, stepId)
		if stepErr != nil {
			return nil, stepErr
		}

		for paramKey, paramVal := range contextParams {
			innerStep, inerStepErr := ae.DB.GetTaskStepByNameForCtxEditing(ctx, taskStep.WorkID, paramKey, taskStep.Time)
			if inerStepErr != nil {
				return nil, inerStepErr
			}

			blockRes, contextErr := ae.editGoBlock(ctx, innerStep.ID, innerStep.Type, paramKey, paramVal, MonitoringTaskEditBlockRequestChangeTypeOutput)
			if contextErr != nil {
				return nil, contextErr
			}

			res = append(res, blockRes...)
		}
	case MonitoringTaskEditBlockRequestChangeTypeInput:
	case MonitoringTaskEditBlockRequestChangeTypeOutput:
		return []EditBlock{{State: map[string]interface{}{}, Output: data, StepName: stepName, StepId: stepId}}, nil

	case MonitoringTaskEditBlockRequestChangeTypeState:
		return []EditBlock{{State: data, Output: map[string]interface{}{}, StepName: stepName, StepId: stepId}}, nil
	}

	return res, nil
}
