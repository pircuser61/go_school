package pipeline

import (
	c "context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/kafka"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/scheduler"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

type FunctionRetryPolicy string

const (
	ErrorKey     = "error"
	KeyDelimiter = "."

	SimpleFunctionRetryPolicy FunctionRetryPolicy = "simple"
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
	CheckSLA       bool                        `json:"check_sla"`
	SLA            int                         `json:"sla"`
	TimeExpired    bool                        `json:"time_expired"`
}

type FunctionUpdateParams struct {
	Action  string                 `json:"action"`
	Mapping map[string]interface{} `json:"mapping"`
}

type ExecutableFunctionBlock struct {
	Name    string
	Title   string
	Input   map[string]string
	Output  map[string]string
	Sockets []script.Socket
	State   *ExecutableFunction
	RunURL  string

	expectedEvents map[string]struct{}
	happenedEvents []entity.NodeEvent

	RunContext *BlockRunContext
}

func (gb *ExecutableFunctionBlock) GetNewEvents() []entity.NodeEvent {
	return gb.happenedEvents
}

func (gb *ExecutableFunctionBlock) Members() []Member {
	return nil
}

func (gb *ExecutableFunctionBlock) Deadlines(_ c.Context) ([]Deadline, error) {
	return []Deadline{}, nil
}

func (gb *ExecutableFunctionBlock) GetStatus() Status {
	if gb.State.TimeExpired {
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

func (gb *ExecutableFunctionBlock) GetTaskHumanStatus() (status TaskHumanStatus, comment string) {
	if gb.State.TimeExpired {
		return StatusDone, ""
	}

	if gb.State.Async && gb.State.HasAck && !gb.State.HasResponse {
		return StatusWait, ""
	}

	if gb.State.HasResponse {
		return StatusDone, ""
	}

	return StatusExecution, ""
}

func (gb *ExecutableFunctionBlock) Next(_ *store.VariableStore) ([]string, bool) {
	key := DefaultSocketID
	if gb.State.TimeExpired {
		key = funcTimeExpired
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

//nolint:gocyclo //its ok here
func (gb *ExecutableFunctionBlock) Update(ctx c.Context) (interface{}, error) {
	log := logger.GetLogger(ctx)

	if gb.RunContext.UpdateData != nil {
		var updateData FunctionUpdateParams
		updateDataUnmarshalErr := json.Unmarshal(gb.RunContext.UpdateData.Parameters, &updateData)
		if updateDataUnmarshalErr != nil {
			return nil, updateDataUnmarshalErr
		}

		log.Info("update function action: " + gb.RunContext.UpdateData.Action)

		switch gb.RunContext.UpdateData.Action {
		case string(entity.TaskUpdateActionFuncSLAExpired):
			gb.State.TimeExpired = true
		default:
			if err := gb.setStateByResponse(&updateData); err != nil {
				return nil, err
			}
		}
	} else {
		if gb.State.HasResponse {
			return nil, nil
		}
		taskStep, errTask := gb.RunContext.Services.Storage.GetTaskStepByName(ctx, gb.RunContext.TaskID, gb.Name)
		if errTask != nil {
			return nil, errTask
		}

		if gb.State.Async {
			isFirstStart, firstStart, errFirstStart := gb.isFirstStart(ctx, gb.RunContext.TaskID, gb.Name)
			if errFirstStart != nil {
				return nil, errFirstStart
			}

			// эта функция уже запускалась и время ожидания корректного ответа закончилось
			if !isFirstStart && firstStart != nil && !isTimeToWaitAnswer(firstStart.Time, gb.State.WaitCorrectRes) {
				em, errEmail := gb.RunContext.Services.People.GetUserEmail(ctx, gb.RunContext.Initiator)
				if errEmail != nil {
					log.WithField("login", gb.RunContext.Initiator).Error(errEmail)
				}

				emails := []string{em}

				tpl := mail.NewInvalidFunctionResp(
					gb.RunContext.WorkNumber, gb.RunContext.NotifName, gb.RunContext.Services.Sender.SdAddress)
				errSend := gb.RunContext.Services.Sender.SendNotification(ctx, emails, nil, tpl)
				if errSend != nil {
					log.WithField("emails", emails).Error(errSend)
				}
			}
		}

		variables, err := getVariables(gb.RunContext.VarStore)
		if err != nil {
			return nil, err
		}

		variables = script.RestoreMapStructure(variables)

		functionMapping, err := script.MapData(gb.State.Mapping, variables, nil)
		if err != nil {
			return nil, err
		}

		if err = gb.fillMapWithConstants(functionMapping); err != nil {
			return nil, err
		}

		if !gb.RunContext.skipProduce {
			err = gb.RunContext.Services.Kafka.Produce(ctx, kafka.RunnerOutMessage{
				TaskID:          taskStep.ID,
				FunctionMapping: functionMapping,
				Contracts:       gb.State.Contracts,
				RetryPolicy:     string(SimpleFunctionRetryPolicy),
				FunctionName:    gb.State.Name,
				FunctionVersion: gb.State.Version,
			})

			if err != nil {
				return nil, err
			}
		}
	}

	var stateBytes []byte
	stateBytes, err := json.Marshal(gb.State)
	if err != nil {
		return nil, err
	}

	gb.RunContext.VarStore.ReplaceState(gb.Name, stateBytes)

	if gb.State.HasResponse || gb.State.TimeExpired {
		if _, ok := gb.expectedEvents[eventEnd]; ok {
			status, _ := gb.GetTaskHumanStatus()
			event, eventErr := gb.RunContext.MakeNodeEndEvent(ctx, gb.Name, status, gb.GetStatus())
			if eventErr != nil {
				return nil, eventErr
			}
			gb.happenedEvents = append(gb.happenedEvents, event)
		}
	}

	return nil, nil
}

func (gb *ExecutableFunctionBlock) fillMapWithConstants(functionMapping map[string]interface{}) error {
	for keyName, value := range gb.State.Constants {
		keyParts := strings.Split(keyName, ".")
		currMap := functionMapping

		for i, part := range keyParts {
			if i == len(keyParts)-1 {
				currMap[part] = value
				break
			}

			newCurrMap, ok := currMap[part]
			if !ok {
				newCurrMap = make(map[string]interface{})
				currMap[part] = newCurrMap
			}

			convNewCurrMap, ok := newCurrMap.(map[string]interface{})
			if !ok {
				return errors.New("can`t assert newCurrMap to map[string]interface{}")
			}

			currMap = convNewCurrMap
		}
	}

	return nil
}

func (gb *ExecutableFunctionBlock) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockExecutableFunctionID,
		BlockType: script.TypeExternal,
		Title:     BlockExecutableFunctionTitle,
		Inputs:    nil,
		Outputs:   nil,
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

// nolint:dupl // another block
func createExecutableFunctionBlock(ctx c.Context, name string, ef *entity.EriusFunc, runCtx *BlockRunContext,
	expectedEvents map[string]struct{}) (*ExecutableFunctionBlock, bool, error) {
	b := &ExecutableFunctionBlock{
		Name:       name,
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
		for propertyName, v := range ef.Output.Properties {
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
		if err := b.createState(ef); err != nil {
			return nil, false, err
		}
		b.RunContext.VarStore.AddStep(b.Name)

		if _, ok := b.expectedEvents[eventStart]; ok {
			status, _ := b.GetTaskHumanStatus()
			event, err := runCtx.MakeNodeStartEvent(ctx, name, status, b.GetStatus())
			if err != nil {
				return nil, false, err
			}
			b.happenedEvents = append(b.happenedEvents, event)
		}
	}

	return b, reEntry, nil
}

func (gb *ExecutableFunctionBlock) loadState(raw json.RawMessage) error {
	return json.Unmarshal(raw, &gb.State)
}

//nolint:dupl,gocyclo //its not duplicate
func (gb *ExecutableFunctionBlock) createState(ef *entity.EriusFunc) error {
	var params script.ExecutableFunctionParams
	err := json.Unmarshal(ef.Params, &params)
	if err != nil {
		return errors.Wrap(err, "can not get executable function parameters")
	}

	if err = params.Validate(); err != nil {
		return errors.Wrap(err, "invalid executable function parameters")
	}

	function, err := gb.RunContext.Services.FunctionStore.GetFunction(c.Background(), params.Function.FunctionId)
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
	}

	if gb.State.CheckSLA {
		_, err = gb.RunContext.Services.Scheduler.CreateTask(c.Background(), &scheduler.CreateTask{
			WorkNumber:  gb.RunContext.WorkNumber,
			WorkID:      gb.RunContext.TaskID.String(),
			ActionName:  string(entity.TaskUpdateActionFuncSLAExpired),
			WaitSeconds: gb.State.SLA,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (gb *ExecutableFunctionBlock) setStateByResponse(updateData *FunctionUpdateParams) error {
	if gb.State.Async && !gb.State.HasAck {
		gb.State.HasAck = true
	} else {
		gb.State.HasResponse = true
	}

	if gb.State.HasResponse {
		var expectedOutput map[string]script.ParamMetadata
		outputUnmarshalErr := json.Unmarshal([]byte(gb.State.Function.Output), &expectedOutput)
		if outputUnmarshalErr != nil {
			return outputUnmarshalErr
		}

		var resultOutput = make(map[string]interface{})

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

		for k, v := range resultOutput {
			gb.RunContext.VarStore.SetValue(gb.Output[k], v)
		}
	}

	return nil
}

func isTimeToWaitAnswer(createdAt time.Time, waitInDays int) bool {
	return time.Now().Before(createdAt.AddDate(0, 0, waitInDays))
}

func (gb *ExecutableFunctionBlock) isFirstStart(ctx c.Context, workId uuid.UUID, sName string) (bool, *entity.Step, error) {
	countRunFunc := 0

	steps, err := gb.RunContext.Services.Storage.GetTaskSteps(ctx, workId)
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
