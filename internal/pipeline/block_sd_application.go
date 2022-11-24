package pipeline

import (
	"context"
	"encoding/json"
	"time"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

const (
	keyOutputBlueprintID       = "blueprint_id"
	keyOutputSdApplicationDesc = "description"
	keyOutputSdApplication     = "application_body"
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
	Name    string
	Title   string
	Input   map[string]string
	Output  map[string]string
	Sockets []script.Socket
	State   *ApplicationData

	RunContext *BlockRunContext
}

func (gb *GoSdApplicationBlock) Members() map[string]struct{} {
	return nil
}

func (gb *GoSdApplicationBlock) CheckSLA() (bool, time.Time) {
	return false, time.Time{}
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

func (gb *GoSdApplicationBlock) GetTaskHumanStatus() TaskHumanStatus {
	return StatusNew
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
	data, err := gb.RunContext.Storage.GetTaskRunContext(ctx, gb.RunContext.WorkNumber)
	if err != nil {
		return nil, errors.Wrap(err, "can't get task run context")
	}

	var appBody map[string]interface{}
	bytes, err := data.InitialApplication.ApplicationBody.MarshalJSON()
	if err != nil {
		return nil, err
	}
	if unmErr := json.Unmarshal(bytes, &appBody); unmErr != nil {
		return nil, err
	}

	gb.RunContext.VarStore.SetValue(gb.Output[keyOutputBlueprintID], gb.State.BlueprintID)
	gb.RunContext.VarStore.SetValue(gb.Output[keyOutputSdApplicationDesc], data.InitialApplication.Description)
	gb.RunContext.VarStore.SetValue(gb.Output[keyOutputSdApplication], appBody)

	gb.State.ApplicationBody = appBody
	gb.State.Description = data.InitialApplication.Description

	var stateBytes []byte
	stateBytes, err = json.Marshal(gb.State)
	if err != nil {
		return nil, err
	}

	gb.RunContext.VarStore.ReplaceState(gb.Name, stateBytes)
	return nil, nil
}

func (gb *GoSdApplicationBlock) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockGoSdApplicationID,
		BlockType: script.TypeGo,
		Title:     BlockGoSdApplicationTitle,
		Inputs:    nil,
		Outputs: []script.FunctionValueModel{
			{
				Name:    keyOutputBlueprintID,
				Type:    "string",
				Comment: "application pipeline id",
			},
			{
				Name:    keyOutputSdApplicationDesc,
				Type:    "string",
				Comment: "application description",
			},
			{
				Name:    keyOutputSdApplication,
				Type:    "object",
				Comment: "application body",
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

func createGoSdApplicationBlock(name string, ef *entity.EriusFunc, runCtx *BlockRunContext) (*GoSdApplicationBlock, error) {
	log := logger.CreateLogger(nil)
	log.WithField("params", ef.Params).Info("sd_application parameters")

	b := &GoSdApplicationBlock{
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

	var params script.SdApplicationParams
	err := json.Unmarshal(ef.Params, &params)
	if err != nil {
		return nil, errors.Wrap(err, "can not get sd_application parameters")
	}

	if err = params.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid sd_application parameters")
	}

	b.State = &ApplicationData{
		BlueprintID: params.BlueprintID,
	}

	b.RunContext.VarStore.AddStep(b.Name)

	return b, nil
}
