package pipeline

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

const (
	keyOutputBlueprintID           = "blueprint_id"
	keyOutputSdApplicationDesc     = "description"
	keyOutputSdApplication         = "application_body"
	keyOutputSdApplicationExecutor = "executor"
)

type ApplicationData struct {
	BlueprintID     string                 `json:"blueprint_id"`
	Description     string                 `json:"description"`
	ApplicationBody map[string]interface{} `json:"application_body"`
}

type SdApplicationData struct {
	Description     string                 `json:"description"`
	ApplicationBody map[string]interface{} `json:"application_body"`
}

type GoSdApplicationBlock struct {
	Name      string
	ShortName string
	Title     string
	Input     map[string]string
	Output    map[string]string
	Sockets   []script.Socket
	State     *ApplicationData

	RunContext *BlockRunContext

	expectedEvents      map[string]struct{}
	happenedEvents      []entity.NodeEvent
	happenedKafkaEvents []entity.NodeKafkaEvent
}

func (gb *GoSdApplicationBlock) CurrentExecutorData() CurrentExecutorData {
	return CurrentExecutorData{}
}

func (gb *GoSdApplicationBlock) GetNewEvents() []entity.NodeEvent {
	return gb.happenedEvents
}

func (gb *GoSdApplicationBlock) GetNewKafkaEvents() []entity.NodeKafkaEvent {
	return gb.happenedKafkaEvents
}

func (gb *GoSdApplicationBlock) Members() []Member {
	return nil
}

func (gb *GoSdApplicationBlock) Deadlines(_ context.Context) ([]Deadline, error) {
	return []Deadline{}, nil
}

func (gb *GoSdApplicationBlock) UpdateManual() bool {
	return false
}

func (gb *GoSdApplicationBlock) GetStatus() Status {
	if gb.State.ApplicationBody != nil {
		return StatusFinished
	}

	return StatusRunning
}

func (gb *GoSdApplicationBlock) GetTaskHumanStatus() (status TaskHumanStatus, comment, action string) {
	return "", "", ""
}

func (gb *GoSdApplicationBlock) Next(_ *store.VariableStore) ([]string, bool) {
	nexts, ok := script.GetNexts(gb.Sockets, DefaultSocketID)
	if !ok {
		return nil, false
	}

	return nexts, true
}

func (gb *GoSdApplicationBlock) GetState() interface{} {
	return gb.State
}

func (gb *GoSdApplicationBlock) Update(ctx context.Context) (interface{}, error) {
	data, err := gb.RunContext.Services.Storage.GetTaskRunContext(ctx, gb.RunContext.WorkNumber)
	if err != nil {
		return nil, errors.Wrap(err, "can't get task run context")
	}

	var appBody map[string]interface{}

	bytes, err := data.InitialApplication.ApplicationBody.MarshalJSON()
	if err != nil {
		return nil, err
	}

	if unmErr := json.Unmarshal(bytes, &appBody); unmErr != nil {
		return nil, unmErr
	}

	ssoUser, err := gb.RunContext.Services.People.GetUser(ctx, gb.RunContext.Initiator, false)
	if err != nil {
		return nil, err
	}

	person, err := ssoUser.ToPerson()
	if err != nil {
		return nil, err
	}

	if valOutputSdApplicationExecutor, ok := gb.Output[keyOutputSdApplicationExecutor]; ok {
		gb.RunContext.VarStore.SetValue(valOutputSdApplicationExecutor, person)
	}

	if valOutputBlueprintID, ok := gb.Output[keyOutputBlueprintID]; ok {
		gb.RunContext.VarStore.SetValue(valOutputBlueprintID, gb.State.BlueprintID)
	}

	if valOutputSdApplicationDesc, ok := gb.Output[keyOutputSdApplicationDesc]; ok {
		gb.RunContext.VarStore.SetValue(valOutputSdApplicationDesc, data.InitialApplication.Description)
	}

	if valOutputSdApplication, ok := gb.Output[keyOutputSdApplication]; ok {
		gb.RunContext.VarStore.SetValue(valOutputSdApplication, appBody)
	}

	gb.State.ApplicationBody = appBody
	gb.State.Description = data.InitialApplication.Description

	var stateBytes []byte

	stateBytes, err = json.Marshal(gb.State)
	if err != nil {
		return nil, err
	}

	gb.RunContext.VarStore.ReplaceState(gb.Name, stateBytes)

	if _, ok := gb.expectedEvents[eventEnd]; ok {
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
	}

	return nil, nil
}

func (gb *GoSdApplicationBlock) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockGoSdApplicationID,
		BlockType: script.TypeGo,
		Title:     BlockGoSdApplicationTitle,
		Inputs:    nil,
		Outputs: &script.JSONSchema{
			Type: "object",
			Properties: script.JSONSchemaProperties{
				keyOutputBlueprintID: {
					Type:        "string",
					Description: "application pipeline id",
				},
				keyOutputSdApplicationDesc: {
					Type:        "string",
					Description: "application description",
				},
				keyOutputSdApplication: {
					Type:        "object",
					Description: "application body",
				},
				keyOutputSdApplicationExecutor: {
					Type:        "object",
					Description: "person object from sso",
					Format:      "SsoPerson",
					Properties:  people.GetSsoPersonSchemaProperties(),
				},
			},
		},
		Params: &script.FunctionParams{
			Type: BlockGoSdApplicationID,
			Params: &script.SdApplicationParams{
				BlueprintID: "",
			},
		},
		Sockets: []script.Socket{script.DefaultSocket},
	}
}

func (gb *GoSdApplicationBlock) BlockAttachments() []string {
	return utils.UniqueStrings(utils.GetAttachmentsIds(fmt.Sprintf("%+v", gb.State.ApplicationBody)))
}

//nolint:unparam // its ok
func createGoSdApplicationBlock(ctx context.Context, name string, ef *entity.EriusFunc, runCtx *BlockRunContext,
	expectedEvents map[string]struct{},
) (*GoSdApplicationBlock, bool, error) {
	log := logger.GetLogger(ctx).WithField("funcName", "createGoSdApplicationBlock")
	log.WithField("params", string(ef.Params)).Info("sd_application parameters")

	const reEntry = false

	b := &GoSdApplicationBlock{
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
			if v.Global == "" {
				continue
			}

			b.Output[propertyName] = v.Global
		}
	}

	var params script.SdApplicationParams

	err := json.Unmarshal(ef.Params, &params)
	if err != nil {
		return nil, reEntry, errors.Wrap(err, "can not get sd_application parameters")
	}

	if err = params.Validate(); err != nil {
		return nil, reEntry, errors.Wrap(err, "invalid sd_application parameters")
	}

	b.State = &ApplicationData{
		BlueprintID: params.BlueprintID,
	}

	b.RunContext.VarStore.AddStep(b.Name)

	if _, ok := b.expectedEvents[eventStart]; ok {
		status, _, _ := b.GetTaskHumanStatus()

		event, err := runCtx.MakeNodeStartEvent(ctx, MakeNodeStartEventArgs{
			NodeName:      name,
			NodeShortName: ef.ShortTitle,
			HumanStatus:   status,
			NodeStatus:    b.GetStatus(),
		})
		if err != nil {
			return nil, false, err
		}

		b.happenedEvents = append(b.happenedEvents, event)
	}

	return b, reEntry, nil
}

type SDOutput struct {
	Executor        people.Person
	ApplicationBody map[string]interface{}
	Description     string
	BlueprintID     string
}

func (gb *GoSdApplicationBlock) UpdateStateUsingOutput(_ context.Context, data []byte) (state map[string]interface{}, err error) {
	sdOutput := SDOutput{}

	unmErr := json.Unmarshal(data, &sdOutput)
	if unmErr != nil {
		return nil, fmt.Errorf("can't unmarshal into output struct")
	}

	gb.State.BlueprintID = sdOutput.BlueprintID
	gb.State.Description = sdOutput.Description

	if sdOutput.ApplicationBody != nil {
		gb.State.ApplicationBody = sdOutput.ApplicationBody
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

func (gb *GoSdApplicationBlock) UpdateOutputUsingState(context.Context) (res map[string]interface{}, err error) {
	output := map[string]interface{}{}

	output[keyOutputBlueprintID] = gb.State.BlueprintID
	output[keyOutputSdApplicationDesc] = gb.State.Description

	if gb.State.ApplicationBody != nil {
		output[keyOutputSdApplication] = gb.State.ApplicationBody
	}

	return output, nil
}
