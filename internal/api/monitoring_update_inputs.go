package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"go.opencensus.io/trace"

	conditions_kit "gitlab.services.mts.ru/jocasta/conditions-kit"

	e "gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/user"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

//nolint:revive,gocritic,stylecheck
func (ae *Env) MonitoringUpdateBlockInputs(w http.ResponseWriter, r *http.Request) {
	const fn = "MonitoringUpdateBlockInputs"

	ctx, span := trace.StartSpan(r.Context(), "monitoring_update_block_inputs")
	defer span.End()

	log := logger.GetLogger(ctx).
		WithField("funcName", "MonitoringUpdateBlockInputs")
	errorHandler := newHTTPErrorHandler(log, w)

	b, err := io.ReadAll(r.Body)

	defer r.Body.Close()

	if err != nil {
		errorHandler.handleError(RequestReadError, err)

		return
	}

	req := &MonitoringUpdateBlockInputsRequest{}

	err = json.Unmarshal(b, req)
	if err != nil {
		errorHandler.handleError(MonitoringEditBlockParseError, err)

		return
	}

	data, err := convertReqEditData(req.Inputs.AdditionalProperties)
	if err != nil {
		log.WithError(err).Error(fmt.Errorf("type and type of value are not compatible: %w", err))
		errorHandler.handleError(TypeAndValueNotCompatible, err)

		return
	}

	err = validateInputs(req.StepName, data)
	if err != nil {
		errorHandler.handleError(MonitoringEditBlockParseError, err)

		return
	}

	log = log.WithField("stepName", req.StepName).
		WithField("workID", req.WorkId)
	ctx = logger.WithLogger(ctx, log)

	ui, err := user.GetUserInfoFromCtx(ctx)
	if err != nil {
		errorHandler.handleError(NoUserInContextError, err)

		return
	}

	eventData := struct {
		Data       map[string]interface{} `json:"data"`
		ChangeType string                 `json:"change_type"`
		StepNames  []string               `json:"step_names"`
	}{
		Data:       data,
		ChangeType: "inputs",
		StepNames:  []string{req.StepName},
	}

	// nolint:ineffassign,staticcheck
	jsonParams := json.RawMessage{}

	jsonParams, err = json.Marshal(eventData)
	if err != nil {
		errorHandler.handleError(MarshalEventParamsError, err)

		return
	}

	txStorage, transactionErr := ae.DB.StartTransaction(ctx)
	if transactionErr != nil {
		log.WithError(transactionErr).Error("couldn't start transaction")

		errorHandler.sendError(UnknownError)

		return
	}

	eventID, err := txStorage.CreateTaskEvent(ctx, &e.CreateTaskEvent{
		WorkID:    req.WorkId,
		Author:    ui.Username,
		EventType: string(MonitoringTaskActionRequestActionEdit),
		Params:    jsonParams,
	})
	if err != nil {
		errorHandler.handleError(CreateTaskEventError, err)
		ae.rollbackTransaction(ctx, txStorage, fn)

		return
	}

	err = txStorage.CreateTaskStepInputs(ctx, &e.CreateTaskStepInputs{
		WorkID:   req.WorkId,
		EventID:  eventID,
		StepName: req.StepName,
		Author:   ui.Username,
		Inputs:   data,
	})
	if err != nil {
		errorHandler.handleError(CreateTaskStepInputsError, err)
		ae.rollbackTransaction(ctx, txStorage, fn)

		return
	}

	err = txStorage.CommitTransaction(ctx)
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}

	getErr := ae.returnInput(w, data)

	if getErr != -1 {
		errorHandler.sendError(getErr)
	}
}

type defaultInputsValidator struct{}

func (defaultInputsValidator) Validate() error {
	return nil
}

func validateInputs(stepName string, inputs map[string]interface{}) (err error) {
	marshData, marshErr := json.Marshal(inputs)
	if marshErr != nil {
		return marshErr
	}

	//nolint:all // stupid bitch I use this variable
	ck := conditions_kit.ConditionParams{}

	blocksInputs := map[string]script.BlockInputsValidator{
		pipeline.BlockGoApproverID:          &script.ApproverParams{},
		pipeline.BlockGoExecutionID:         &script.ExecutionParams{},
		pipeline.BlockExecutableFunctionID:  &script.FunctionParams{},
		pipeline.BlockGoFormID:              &script.FormParams{},
		pipeline.BlockGoNotificationID:      &script.NotificationParams{},
		pipeline.BlockGoSdApplicationID:     &script.SdApplicationParams{},
		pipeline.BlockGoSignID:              &script.SignParams{},
		pipeline.BlockGoStartID:             &defaultInputsValidator{},
		pipeline.BlockGoEndID:               &defaultInputsValidator{},
		pipeline.BlockGoBeginParallelTaskID: &defaultInputsValidator{},
		pipeline.BlockWaitForAllInputsID:    &defaultInputsValidator{},
		pipeline.BlockGoIfID:                &ck,
	}

	stepType := regexp.MustCompile(`_\d+`).ReplaceAllString(stepName, "")
	if _, ok := blocksInputs[stepType]; !ok {
		return fmt.Errorf("unknown block type %s", stepType)
	}

	stepParams := blocksInputs[stepType]

	err = json.Unmarshal(marshData, &stepParams)
	if err != nil {
		return err
	}

	err = stepParams.Validate()
	if err != nil {
		return err
	}

	return nil
}

func (ae *Env) returnInput(w http.ResponseWriter, in map[string]interface{}) (getErr Err) {
	inputs := make(map[string]MonitoringEditBlockData, 0)

	for k, v := range in {
		inputs[k] = MonitoringEditBlockData{
			Name:  k,
			Value: v,
			Type:  utils.GetJSONType(v),
		}
	}

	if err := sendResponse(w, http.StatusOK, BlockEditResponse{
		Blocks: &BlockEditResponse_Blocks{AdditionalProperties: inputs},
	}); err != nil {
		return UnknownError
	}

	return -1
}
