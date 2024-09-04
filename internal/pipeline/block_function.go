package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/scheduler"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

type FunctionRetryPolicy string

type FunctionDecision string

const (
	keyOutputFunctionDecision = "decision"

	KeyDelimiter = "."

	SimpleFunctionRetryPolicy FunctionRetryPolicy = "simple"

	TimeoutDecision            FunctionDecision = "timeout"
	ExecutedDecision           FunctionDecision = "executed"
	RetryCountExceededDecision FunctionDecision = "retry_count_exceeded"
)

type ExecutableFunction struct {
	Name           string                      `json:"name"`
	Version        string                      `json:"version"`
	Mapping        script.JSONSchemaProperties `json:"mapping"`
	Function       script.FunctionParam        `json:"function"`
	Async          bool                        `json:"async"`
	HasAck         bool                        `json:"has_ack"`
	HasResponse    bool                        `json:"has_response"`
	Contracts      string                      `json:"contracts"`
	WaitCorrectRes int                         `json:"waitCorrectRes"`
	Constants      map[string]interface{}      `json:"constants"`

	// SLA
	CheckSLA bool `json:"check_sla"`
	SLA      int  `json:"sla"`

	TimeExpired bool `json:"time_expired"`

	Started bool `json:"started"`

	// Retry
	RetryPolicy        script.FunctionRetryPolicy `json:"retry_policy"`
	RetryCount         int                        `json:"retry_count"`
	CurrRetryCount     int                        `json:"cur_retry_count"`
	CurrRetryTimeout   int                        `json:"cur_retry_timeout"`
	RetryTimeouts      []int                      `json:"retry_timeouts"` // for timeout's calculations
	RetryCountExceeded bool                       `json:"retry_count_exceeded"`
}

func NewExecutableFunctionState() *ExecutableFunction {
	return &ExecutableFunction{
		Mapping:       make(map[string]script.JSONSchemaPropertiesValue, 0),
		Constants:     make(map[string]interface{}, 0),
		RetryTimeouts: make([]int, 0),
	}
}

type FunctionUpdateParams struct {
	Action        string                 `json:"action"`
	Mapping       map[string]interface{} `json:"mapping"`
	Err           string                 `json:"err"`
	DoRetry       bool                   `json:"do_retry"`
	IsAsyncResult bool                   `json:"is_async_result"`
}

type ExecutableFunctionBlock struct {
	Name      string
	ShortName string
	Title     string
	Input     map[string]string
	Output    map[string]string
	Sockets   []script.Socket
	State     *ExecutableFunction
	RunURL    string

	expectedEvents      map[string]struct{}
	happenedEvents      []entity.NodeEvent
	happenedKafkaEvents []entity.NodeKafkaEvent

	RunContext *BlockRunContext
}

func (gb *ExecutableFunctionBlock) CurrentExecutorData() CurrentExecutorData {
	return CurrentExecutorData{}
}

func (gb *ExecutableFunctionBlock) GetNewEvents() []entity.NodeEvent {
	return gb.happenedEvents
}

func (gb *ExecutableFunctionBlock) GetNewKafkaEvents() []entity.NodeKafkaEvent {
	return gb.happenedKafkaEvents
}

func (gb *ExecutableFunction) GetSchema() string {
	// Было -> [str1 str2] | Стало -> ["str1" "str2"]
	required := fmt.Sprintf("%q", gb.Function.RequiredInput)
	// Было ["str1" "str2"] | Стало -> ["str1","str2"]
	required = strings.ReplaceAll(required, " ", ",")

	return fmt.Sprintf(`{"type": "object", "properties": %s, "required": %s}`, gb.Function.Input, required)
}

func (gb *ExecutableFunctionBlock) Members() []Member {
	return nil
}

func (gb *ExecutableFunctionBlock) Deadlines(_ context.Context) ([]Deadline, error) {
	return []Deadline{}, nil
}

func (gb *ExecutableFunctionBlock) GetStatus() Status {
	if gb.State.TimeExpired || gb.State.RetryCountExceeded {
		return StatusFinished
	}

	if gb.State.Async && gb.State.HasAck && !gb.State.HasResponse {
		return StatusIdle
	}

	if gb.State.HasResponse {
		return StatusFinished
	}

	return StatusRunning
}

func (gb *ExecutableFunctionBlock) GetTaskHumanStatus() (status TaskHumanStatus, comment, action string) {
	if gb.State.TimeExpired || gb.State.RetryCountExceeded {
		return StatusDone, "", ""
	}

	if gb.State.Async && gb.State.HasAck && !gb.State.HasResponse {
		return StatusWait, "", ""
	}

	if gb.State.HasResponse {
		return StatusDone, "", ""
	}

	return StatusExecution, "", ""
}

func (gb *ExecutableFunctionBlock) Next(_ *store.VariableStore) ([]string, bool) {
	key := DefaultSocketID
	if gb.State.TimeExpired {
		key = funcTimeExpired
	}

	if gb.State.RetryCountExceeded {
		key = retryCountExceeded
	}

	nexts, ok := script.GetNexts(gb.Sockets, key)
	if !ok {
		return nil, false
	}

	return nexts, true
}

func (gb *ExecutableFunctionBlock) GetState() interface{} {
	return gb.State
}

func (gb *ExecutableFunctionBlock) Update(ctx context.Context) (interface{}, error) {
	log := logger.GetLogger(ctx)

	if gb.RunContext.UpdateData != nil {
		err := gb.updateFunctionResult(ctx, log)
		if err != nil {
			return nil, err
		}
	} else {
		err := gb.runFunction(ctx, log)
		if err != nil {
			return nil, err
		}
	}

	var stateBytes []byte

	stateBytes, err := json.Marshal(gb.State)
	if err != nil {
		return nil, err
	}

	gb.RunContext.VarStore.ReplaceState(gb.Name, stateBytes)

	if gb.State.HasResponse || gb.State.TimeExpired || gb.State.RetryCountExceeded {
		_, ok := gb.expectedEvents[eventEnd]
		if !ok {
			return nil, nil
		}

		status, _, _ := gb.GetTaskHumanStatus()

		event, eventErr := gb.RunContext.MakeNodeEndEvent(ctx, MakeNodeEndEventArgs{
			NodeName:      gb.Name,
			NodeShortName: gb.ShortName,
			HumanStatus:   status,
			NodeStatus:    gb.GetStatus(),
		})
		if eventErr != nil {
			return nil, eventErr
		}

		gb.happenedEvents = append(gb.happenedEvents, event)

		// delete those that may exist
		err := gb.RunContext.Services.Scheduler.DeleteTask(ctx,
			&scheduler.DeleteTask{
				WorkID:   gb.RunContext.TaskID.String(),
				StepName: gb.Name,
			})
		if err != nil {
			log.WithError(err).Error("cannot delete scheduler task for function")

			return nil, err
		}
	}

	return nil, nil
}

func (gb *ExecutableFunctionBlock) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockExecutableFunctionID,
		BlockType: script.TypeExternal,
		Title:     BlockExecutableFunctionTitle,
		Inputs:    nil,
		Outputs: &script.JSONSchema{
			Type: "object",
			Properties: script.JSONSchemaProperties{
				keyOutputFunctionDecision: {
					Type:        "string",
					Title:       "Решение",
					Description: "function decision",
				},
			},
		},
		Params: &script.FunctionParams{
			Type: BlockExecutableFunctionID,
			Params: &script.ExecutableFunctionParams{
				Name:    "",
				Version: "",
				Mapping: script.JSONSchemaProperties{},
			},
		},
		Sockets: []script.Socket{
			script.FuncExecutedSocket,
		},
	}
}

func (gb *ExecutableFunctionBlock) UpdateManual() bool {
	return false
}

func (gb *ExecutableFunctionBlock) BlockAttachments() (ids []string) {
	return ids
}

// nolint:dupl // another block
func createExecutableFunctionBlock(ctx context.Context, name string, ef *entity.EriusFunc, runCtx *BlockRunContext,
	expectedEvents map[string]struct{},
) (*ExecutableFunctionBlock, bool, error) {
	b := &ExecutableFunctionBlock{
		Name:       name,
		ShortName:  ef.ShortTitle,
		Title:      ef.Title,
		Input:      map[string]string{},
		Output:     map[string]string{},
		Sockets:    entity.ConvertSocket(ef.Sockets),
		RunContext: runCtx,
		State:      NewExecutableFunctionState(),

		expectedEvents: expectedEvents,
		happenedEvents: make([]entity.NodeEvent, 0),
	}

	for _, v := range ef.Input {
		b.Input[v.Name] = v.Global
	}

	if ef.Output != nil {
		//nolint:gocritic //в этом проекте не принято использовать поинтеры в коллекциях
		for propertyName, v := range ef.Output.Properties {
			if v.Global == "" {
				continue
			}

			b.Output[propertyName] = v.Global
		}
	}

	rawState, blockExists := runCtx.VarStore.State[name]

	reEntry := blockExists && runCtx.UpdateData == nil
	if blockExists && !reEntry {
		if err := b.loadState(rawState); err != nil {
			return nil, false, err
		}
	} else {
		err := b.createExpectedEvents(ctx, runCtx, name, ef)
		if err != nil {
			return nil, false, err
		}
	}

	return b, reEntry, nil
}

func (gb *ExecutableFunctionBlock) loadState(raw json.RawMessage) error {
	return json.Unmarshal(raw, &gb.State)
}

//nolint:dupl //its not duplicate
func (gb *ExecutableFunctionBlock) createState(ctx context.Context, ef *entity.EriusFunc) error {
	var params script.ExecutableFunctionParams

	err := json.Unmarshal(ef.Params, &params)
	if err != nil {
		return errors.Wrap(err, "can not get executable function parameters")
	}

	if err = params.Validate(); err != nil {
		return errors.Wrap(err, "invalid executable function parameters")
	}

	function, err := gb.RunContext.Services.FunctionStore.GetFunctionVersion(
		ctx,
		params.Function.FunctionID,
		params.Function.VersionID,
	)
	if err != nil {
		return err
	}

	isAsync, invalidOptionTypeErr := function.IsAsync()
	if invalidOptionTypeErr != nil {
		return invalidOptionTypeErr
	}

	gb.State = &ExecutableFunction{
		Name:           params.Name,
		Version:        params.Version,
		Mapping:        params.Mapping,
		Function:       params.Function,
		HasAck:         false,
		HasResponse:    false,
		Async:          isAsync,
		Contracts:      function.Contracts,
		WaitCorrectRes: params.WaitCorrectRes,
		Constants:      params.Constants,
		CheckSLA:       params.CheckSLA,
		SLA:            params.SLA,
		RetryTimeouts:  make([]int, 0),
	}

	if params.NeedRetry {
		gb.State.RetryPolicy = params.RetryPolicy
		gb.State.RetryCount = params.RetryCount
		gb.State.CurrRetryTimeout = params.RetryInterval
		gb.State.RetryTimeouts = make([]int, 0)
	}

	if gb.State.CheckSLA {
		_, err = gb.RunContext.Services.Scheduler.CreateTask(ctx, &scheduler.CreateTask{
			WorkNumber:  gb.RunContext.WorkNumber,
			WorkID:      gb.RunContext.TaskID.String(),
			ActionName:  string(entity.TaskUpdateActionFuncSLAExpired),
			StepName:    gb.Name,
			WaitSeconds: gb.State.SLA,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (gb *ExecutableFunctionBlock) createExpectedEvents(
	ctx context.Context,
	runCtx *BlockRunContext,
	name string,
	ef *entity.EriusFunc,
) error {
	if err := gb.createState(ctx, ef); err != nil {
		return err
	}

	gb.RunContext.VarStore.AddStep(gb.Name)

	if _, ok := gb.expectedEvents[eventStart]; ok {
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
	}

	return nil
}

var ErrMessageFromKafkaHasError = errors.New("message from kafka has error")

//nolint:gocognit,gocyclo //it's ok
func (gb *ExecutableFunctionBlock) setStateByResponse(ctx context.Context, log logger.Logger, updateData *FunctionUpdateParams) error {
	if gb.State.Async && gb.State.HasAck && !updateData.IsAsyncResult {
		log.Warning("repeating ack message")

		return nil
	}

	//nolint:nestif //it's ok
	if updateData.DoRetry && gb.State.RetryCount > 0 {
		if gb.State.CurrRetryCount >= gb.State.RetryCount {
			if valOutputFunctionDecision, ok := gb.Output[keyOutputFunctionDecision]; ok {
				gb.RunContext.VarStore.SetValue(valOutputFunctionDecision, RetryCountExceededDecision)
			}

			gb.State.RetryCountExceeded = true
		} else if !gb.RunContext.skipProduce { // for test
			gb.State.Started = false

			_, err := gb.RunContext.Services.Scheduler.CreateTask(ctx, &scheduler.CreateTask{
				WorkNumber:  gb.RunContext.WorkNumber,
				WorkID:      gb.RunContext.TaskID.String(),
				ActionName:  string(entity.TaskUpdateActionRetry),
				StepName:    gb.Name,
				WaitSeconds: gb.State.CurrRetryTimeout,
			})
			if err != nil {
				return err
			}
		}

		return nil
	}

	if updateData.Err != "" {
		log.WithField("message.Err", updateData.Err).
			Error("message from kafka has error")

		return ErrMessageFromKafkaHasError
	}

	if gb.State.Async && !gb.State.HasAck && !updateData.IsAsyncResult {
		gb.State.HasAck = true
	} else {
		gb.State.HasAck = true
		gb.State.HasResponse = true
	}

	//nolint:nestif //it's ok
	if gb.State.HasResponse {
		var expectedOutput map[string]script.ParamMetadata

		outputUnmarshalErr := json.Unmarshal([]byte(gb.State.Function.Output), &expectedOutput)
		if outputUnmarshalErr != nil {
			return outputUnmarshalErr
		}

		resultOutput := make(map[string]interface{})

		for k := range expectedOutput {
			param, ok := updateData.Mapping[k]
			if !ok {
				continue
			}
			// We're using pointer because we sometimes need to change type inside interface
			// from float to integer (func simpleTypeHandler)
			if err := utils.CheckVariableType(&param, expectedOutput[k]); err != nil {
				return err
			}

			resultOutput[k] = param
		}

		gb.RunContext.VarStore.ClearValues(gb.Name)

		if valOutputFunctionDecision, ok := gb.Output[keyOutputFunctionDecision]; ok {
			gb.RunContext.VarStore.SetValue(valOutputFunctionDecision, ExecutedDecision)
		}

		for k, v := range resultOutput {
			if valFromOutput, ok := gb.Output[k]; ok {
				gb.RunContext.VarStore.SetValue(valFromOutput, v)
			}
		}
	}

	return nil
}

func isTimeToWaitAnswer(createdAt time.Time, waitInDays int) bool {
	return time.Now().Before(createdAt.AddDate(0, 0, waitInDays))
}

func (gb *ExecutableFunctionBlock) isFirstStart(ctx context.Context, workID uuid.UUID, sName string) (bool, *entity.Step, error) {
	countRunFunc := 0

	steps, err := gb.RunContext.Services.Storage.GetTaskSteps(ctx, workID)
	if err != nil {
		return false, nil, err
	}

	var firstRun *entity.Step

	sort.Slice(steps, func(i, j int) bool {
		return steps[i].Time.Before(steps[j].Time)
	})

	for i := range steps {
		if steps[i].Name == sName {
			countRunFunc++

			if firstRun == nil {
				firstRun = steps[i]
			}
		}
	}

	return countRunFunc > 1, firstRun, nil
}

func (gb *ExecutableFunctionBlock) UpdateStateUsingOutput(_ context.Context, data []byte) (state map[string]interface{}, err error) {
	output := make(map[string]interface{})

	unmErr := json.Unmarshal(data, &output)
	if unmErr != nil {
		return nil, fmt.Errorf("can't unmarshal into output struct")
	}

	decision, ok := output[keyOutputFunctionDecision]
	if ok {
		switch decision {
		case string(TimeoutDecision):
			gb.State.TimeExpired = true
			gb.State.RetryCountExceeded = false
			gb.State.HasResponse = false
		case string(ExecutedDecision):
			gb.State.TimeExpired = false
			gb.State.RetryCountExceeded = false
			gb.State.HasResponse = true
		case string(RetryCountExceededDecision):
			gb.State.TimeExpired = false
			gb.State.RetryCountExceeded = true
			gb.State.HasResponse = false
		}
	}

	jsonState, marshErr := json.Marshal(gb.State)
	if marshErr != nil {
		return nil, marshErr
	}

	unmarshErr := json.Unmarshal(jsonState, &state)
	if unmarshErr != nil {
		return nil, unmarshErr
	}

	return state, nil
}

func (gb *ExecutableFunctionBlock) UpdateOutputUsingState(context.Context) (output map[string]interface{}, err error) {
	output = make(map[string]interface{})

	switch {
	case gb.State.TimeExpired:
		output[keyOutputFunctionDecision] = TimeoutDecision
	case gb.State.HasResponse:
		output[keyOutputFunctionDecision] = ExecutedDecision
	case gb.State.RetryCountExceeded:
		output[keyOutputFunctionDecision] = RetryCountExceededDecision
	}

	return output, nil
}
