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

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

type FunctionOutput struct {
	//
}

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

	_, editErr := ae.editGoBlock(ctx, blockUUID, dbStep.Type, dbStep.Name, data, req.ChangeType)
	if editErr != nil {
		errorHandler.handleError(EditMonitoringBlockError, err)
	}

	return
}

func convertReqEditData(reqData map[string]MonitoringEditBlockData) (convertedData map[string]interface{}) {
	for key, val := range reqData {
		convertedData[key] = val.Value
	}
	return convertedData
}

type EditBlock struct {
	StepId        uuid.UUID
	stepName      string
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
		// взять ноду по префиксу первую чать по сплиту
		// по ключу пониммаем тип ноды(! executable_func)

		// определяем все префиксы
		// пример {
		// fjrm_0.executor = test
		// form_0.desicion = ok
		// approver_0.desicion = reject
		// }
		//  хочу получить form_0: map{executor = test, desicion = ok}
		// такая же для аппрувера
		// потом идем по форм0, аппрувер 0, определяем тип ноды и идем циклом для этого типа и мапы вызываем editGoBlock(передаем найденный степ айди, степнейм и тд)
		// проходим и собираем по каждому префикс все аутпуты в мапу

		// п

		// по степнейму надо определить stepId ближайший к себе блок с таким степнеймом

		contextParams := map[string]map[string]interface{}{}

		for key, val := range data {
			splitedCtxParam := strings.Split(key, ".")
			contextParams[splitedCtxParam[0]] = map[string]interface{}{splitedCtxParam[1]: val}
		}

		taskStep, stepErr := ae.DB.GetTaskStepByID(ctx, stepId)
		if stepErr != nil {
			return nil, stepErr
		}

		for paramKey, paramVal := range contextParams {
			innerStep, inerStepErr := ae.DB.GetTaskStepByName(ctx, taskStep.WorkID, paramKey)
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

		blockState, stateErr := ae.DB.GetRawBlockState(ctx, stepId.String())
		if stateErr != nil {
			return nil, stateErr
		}

		marshBlockData, marshBlockErr := json.Marshal(blockState)
		if marshBlockErr != nil {
			return nil, marshBlockErr
		}

		approverState := pipeline.ApproverData{}

		unmErr := json.Unmarshal(marshBlockData, &approverState)
		if unmErr != nil {
			return nil, unmErr
		}

		block := pipeline.GoApproverBlock{State: &approverState}

		updState, updErr := block.UpdateStateUsingOutput(ctx, marshData)
		if updErr != nil {
			return nil, updErr
		}

		return []EditBlock{{State: updState, Output: data, stepName: stepName, StepId: stepId}}, nil

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

		return []EditBlock{{State: data, Output: updOutput, stepName: stepName, StepId: stepId}}, nil
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
			contextParams[splitedCtxParam[0]] = map[string]interface{}{splitedCtxParam[1]: val}
		}

		taskStep, stepErr := ae.DB.GetTaskStepByID(ctx, stepId)
		if stepErr != nil {
			return nil, stepErr
		}

		for paramKey, paramVal := range contextParams {
			innerStep, inerStepErr := ae.DB.GetTaskStepByName(ctx, taskStep.WorkID, paramKey)
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

		blockState, stateErr := ae.DB.GetRawBlockState(ctx, stepId.String())
		if stateErr != nil {
			return nil, stateErr
		}

		marshBlockData, marshBlockErr := json.Marshal(blockState)
		if marshBlockErr != nil {
			return nil, marshBlockErr
		}

		execState := pipeline.ExecutionData{}

		unmErr := json.Unmarshal(marshBlockData, &execState)
		if unmErr != nil {
			return nil, unmErr
		}

		block := pipeline.GoExecutionBlock{State: &execState}

		updState, updErr := block.UpdateStateUsingOutput(ctx, marshData)
		if updErr != nil {
			return nil, updErr
		}

		return []EditBlock{{State: updState, Output: data, stepName: stepName, StepId: stepId}}, nil

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

		return []EditBlock{{State: data, Output: updOutput, stepName: stepName, StepId: stepId}}, nil
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
			contextParams[splitedCtxParam[0]] = map[string]interface{}{splitedCtxParam[1]: val}
		}

		taskStep, stepErr := ae.DB.GetTaskStepByID(ctx, stepId)
		if stepErr != nil {
			return nil, stepErr
		}

		for paramKey, paramVal := range contextParams {
			innerStep, inerStepErr := ae.DB.GetTaskStepByName(ctx, taskStep.WorkID, paramKey)
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

		// Нет нужной структуры для валидации

	case MonitoringTaskEditBlockRequestChangeTypeOutput:

		return []EditBlock{{State: map[string]interface{}{}, Output: data, stepName: stepName, StepId: stepId}}, nil

	case MonitoringTaskEditBlockRequestChangeTypeState:

		return []EditBlock{{State: data, Output: map[string]interface{}{}, stepName: stepName, StepId: stepId}}, nil

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
			contextParams[splitedCtxParam[0]] = map[string]interface{}{splitedCtxParam[1]: val}
		}

		taskStep, stepErr := ae.DB.GetTaskStepByID(ctx, stepId)
		if stepErr != nil {
			return nil, stepErr
		}

		for paramKey, paramVal := range contextParams {
			innerStep, inerStepErr := ae.DB.GetTaskStepByName(ctx, taskStep.WorkID, paramKey)
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

		// нет структуры

	case MonitoringTaskEditBlockRequestChangeTypeOutput:

		return []EditBlock{{State: map[string]interface{}{}, Output: data, stepName: stepName, StepId: stepId}}, nil

	case MonitoringTaskEditBlockRequestChangeTypeState:

		return []EditBlock{{State: data, Output: map[string]interface{}{}, stepName: stepName, StepId: stepId}}, nil

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
			contextParams[splitedCtxParam[0]] = map[string]interface{}{splitedCtxParam[1]: val}
		}

		taskStep, stepErr := ae.DB.GetTaskStepByID(ctx, stepId)
		if stepErr != nil {
			return nil, stepErr
		}

		for paramKey, paramVal := range contextParams {
			innerStep, inerStepErr := ae.DB.GetTaskStepByName(ctx, taskStep.WorkID, paramKey)
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

	// no struct

	case MonitoringTaskEditBlockRequestChangeTypeOutput:

		return []EditBlock{{State: map[string]interface{}{}, Output: data, stepName: stepName, StepId: stepId}}, nil

	case MonitoringTaskEditBlockRequestChangeTypeState:

		return []EditBlock{{State: data, Output: map[string]interface{}{}, stepName: stepName, StepId: stepId}}, nil

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
			contextParams[splitedCtxParam[0]] = map[string]interface{}{splitedCtxParam[1]: val}
		}

		taskStep, stepErr := ae.DB.GetTaskStepByID(ctx, stepId)
		if stepErr != nil {
			return nil, stepErr
		}

		for paramKey, paramVal := range contextParams {
			innerStep, inerStepErr := ae.DB.GetTaskStepByName(ctx, taskStep.WorkID, paramKey)
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

	// no struct

	case MonitoringTaskEditBlockRequestChangeTypeOutput:

		return []EditBlock{{State: map[string]interface{}{}, Output: data, stepName: stepName, StepId: stepId}}, nil

	case MonitoringTaskEditBlockRequestChangeTypeState:

		return []EditBlock{{State: data, Output: map[string]interface{}{}, stepName: stepName, StepId: stepId}}, nil

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
			contextParams[splitedCtxParam[0]] = map[string]interface{}{splitedCtxParam[1]: val}
		}

		taskStep, stepErr := ae.DB.GetTaskStepByID(ctx, stepId)
		if stepErr != nil {
			return nil, stepErr
		}

		for paramKey, paramVal := range contextParams {
			innerStep, inerStepErr := ae.DB.GetTaskStepByName(ctx, taskStep.WorkID, paramKey)
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

		// no validate method

		// validateErr := funcParams.Validate()
		// if validateErr != nil {
		// 	return validateErr
		// }

	case MonitoringTaskEditBlockRequestChangeTypeOutput:

		funcParams := FunctionOutput{}

		unmErr := json.Unmarshal(marshData, &funcParams)
		if unmErr != nil {
			return nil, fmt.Errorf("can't unmarshal into output struct")
		}

	case MonitoringTaskEditBlockRequestChangeTypeState:

		execParams := pipeline.ExecutableFunction{}

		unmErr := json.Unmarshal(marshData, &execParams)
		if unmErr != nil {
			return nil, fmt.Errorf("can't unmarshal into state struct")
		}
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
			contextParams[splitedCtxParam[0]] = map[string]interface{}{splitedCtxParam[1]: val}
		}

		taskStep, stepErr := ae.DB.GetTaskStepByID(ctx, stepId)
		if stepErr != nil {
			return nil, stepErr
		}

		for paramKey, paramVal := range contextParams {
			innerStep, inerStepErr := ae.DB.GetTaskStepByName(ctx, taskStep.WorkID, paramKey)
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

		blockState, stateErr := ae.DB.GetRawBlockState(ctx, stepId.String())
		if stateErr != nil {
			return nil, stateErr
		}

		marshBlockData, marshBlockErr := json.Marshal(blockState)
		if marshBlockErr != nil {
			return nil, marshBlockErr
		}

		formBlockState := pipeline.FormData{}

		unmErr := json.Unmarshal(marshBlockData, &formBlockState)
		if unmErr != nil {
			return nil, unmErr
		}

		block := pipeline.GoFormBlock{State: &formBlockState}

		updState, updErr := block.UpdateStateUsingOutput(ctx, marshData)
		if updErr != nil {
			return nil, updErr
		}

		return []EditBlock{{State: updState, Output: data, stepName: stepName, StepId: stepId}}, nil

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

		return []EditBlock{{State: data, Output: updOutput, stepName: stepName, StepId: stepId}}, nil
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
			contextParams[splitedCtxParam[0]] = map[string]interface{}{splitedCtxParam[1]: val}
		}

		taskStep, stepErr := ae.DB.GetTaskStepByID(ctx, stepId)
		if stepErr != nil {
			return nil, stepErr
		}

		for paramKey, paramVal := range contextParams {
			innerStep, inerStepErr := ae.DB.GetTaskStepByName(ctx, taskStep.WorkID, paramKey)
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

		// no struct

	case MonitoringTaskEditBlockRequestChangeTypeOutput:

		return []EditBlock{{State: map[string]interface{}{}, Output: data, stepName: stepName, StepId: stepId}}, nil

	case MonitoringTaskEditBlockRequestChangeTypeState:

		return []EditBlock{{State: data, Output: map[string]interface{}{}, stepName: stepName, StepId: stepId}}, nil

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
			contextParams[splitedCtxParam[0]] = map[string]interface{}{splitedCtxParam[1]: val}
		}

		taskStep, stepErr := ae.DB.GetTaskStepByID(ctx, stepId)
		if stepErr != nil {
			return nil, stepErr
		}

		for paramKey, paramVal := range contextParams {
			innerStep, inerStepErr := ae.DB.GetTaskStepByName(ctx, taskStep.WorkID, paramKey)
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

		return []EditBlock{{State: map[string]interface{}{}, Output: data, stepName: stepName, StepId: stepId}}, nil

	case MonitoringTaskEditBlockRequestChangeTypeState:

		return []EditBlock{{State: data, Output: map[string]interface{}{}, stepName: stepName, StepId: stepId}}, nil

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
			contextParams[splitedCtxParam[0]] = map[string]interface{}{splitedCtxParam[1]: val}
		}

		taskStep, stepErr := ae.DB.GetTaskStepByID(ctx, stepId)
		if stepErr != nil {
			return nil, stepErr
		}

		for paramKey, paramVal := range contextParams {
			innerStep, inerStepErr := ae.DB.GetTaskStepByName(ctx, taskStep.WorkID, paramKey)
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

		blockState, stateErr := ae.DB.GetRawBlockState(ctx, stepId.String())
		if stateErr != nil {
			return nil, stateErr
		}

		marshBlockData, marshBlockErr := json.Marshal(blockState)
		if marshBlockErr != nil {
			return nil, marshBlockErr
		}

		sdState := pipeline.ApplicationData{}

		unmErr := json.Unmarshal(marshBlockData, &sdState)
		if unmErr != nil {
			return nil, unmErr
		}

		block := pipeline.GoSdApplicationBlock{State: &sdState}

		updState, updErr := block.UpdateStateUsingOutput(ctx, marshData)
		if updErr != nil {
			return nil, updErr
		}

		return []EditBlock{{State: updState, Output: data, stepName: stepName, StepId: stepId}}, nil

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

		return []EditBlock{{State: data, Output: updOutput, stepName: stepName, StepId: stepId}}, nil
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
			contextParams[splitedCtxParam[0]] = map[string]interface{}{splitedCtxParam[1]: val}
		}

		for paramKey, paramVal := range contextParams {
			innerStep, inerStepErr := ae.DB.GetTaskStepByName(ctx, taskStep.WorkID, paramKey)
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

		blockState, stateErr := ae.DB.GetRawBlockState(ctx, stepId.String())
		if stateErr != nil {
			return nil, stateErr
		}

		marshBlockData, marshBlockErr := json.Marshal(blockState)
		if marshBlockErr != nil {
			return nil, marshBlockErr
		}

		signState := pipeline.SignData{}

		unmErr := json.Unmarshal(marshBlockData, &signState)
		if unmErr != nil {
			return nil, unmErr
		}

		block := pipeline.GoSignBlock{State: &signState}

		updState, updErr := block.UpdateStateUsingOutput(ctx, marshData)
		if updErr != nil {
			return nil, updErr
		}

		return []EditBlock{{State: updState, Output: data, stepName: stepName, StepId: stepId}}, nil

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

		return []EditBlock{{State: data, Output: updOutput, stepName: stepName, StepId: stepId}}, nil

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
			contextParams[splitedCtxParam[0]] = map[string]interface{}{splitedCtxParam[1]: val}
		}

		taskStep, stepErr := ae.DB.GetTaskStepByID(ctx, stepId)
		if stepErr != nil {
			return nil, stepErr
		}

		for paramKey, paramVal := range contextParams {
			innerStep, inerStepErr := ae.DB.GetTaskStepByName(ctx, taskStep.WorkID, paramKey)
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

		// no struct

	case MonitoringTaskEditBlockRequestChangeTypeOutput:

		return []EditBlock{{State: map[string]interface{}{}, Output: data, stepName: stepName, StepId: stepId}}, nil

	case MonitoringTaskEditBlockRequestChangeTypeState:

		return []EditBlock{{State: data, Output: map[string]interface{}{}, stepName: stepName, StepId: stepId}}, nil
	}

	return res, nil
}
