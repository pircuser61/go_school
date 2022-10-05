package pipeline

import (
	"context"
	"encoding/json"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

const (
	keyOutputBlueprintID       = "blueprint_id"
	keyOutputSdApplicationDesc = "description"
	keyOutputSdApplication     = "application_body"
)

type SdApplicationDataCtx struct{}

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

func (gb *GoSdApplicationBlock) GetType() string {
	return BlockGoSdApplicationID
}

func (gb *GoSdApplicationBlock) Inputs() map[string]string {
	return gb.Input
}

func (gb *GoSdApplicationBlock) Outputs() map[string]string {
	return gb.Output
}

func (gb *GoSdApplicationBlock) IsScenario() bool {
	return false
}

func (gb *GoSdApplicationBlock) DebugRun(ctx context.Context, _ *stepCtx, runCtx *store.VariableStore) (err error) {
	_, s := trace.StartSpan(ctx, "run_go_sd_block")
	defer s.End()

	log := logger.CreateLogger(nil)

	runCtx.AddStep(gb.Name)

	data := ctx.Value(SdApplicationDataCtx{})
	if data == nil {
		return errors.New("can't find application data in context")
	}

	appData, ok := data.(SdApplicationData)
	if !ok {
		return errors.New("invalid application data in context")
	}

	log.WithField("blueprintID", gb.State.BlueprintID).Info("run sd_application block")

	runCtx.SetValue(gb.Output[keyOutputBlueprintID], gb.State.BlueprintID)
	runCtx.SetValue(gb.Output[keyOutputSdApplicationDesc], appData.Description)
	runCtx.SetValue(gb.Output[keyOutputSdApplication], appData.ApplicationBody)

	gb.State.ApplicationBody = appData.ApplicationBody
	gb.State.Description = appData.Description

	var stateBytes []byte
	stateBytes, err = json.Marshal(gb.State)
	if err != nil {
		return err
	}

	runCtx.ReplaceState(gb.Name, stateBytes)

	return err
}

func (gb *GoSdApplicationBlock) Next(_ *store.VariableStore) ([]string, bool) {
	nexts, ok := script.GetNexts(gb.Sockets, DefaultSocketID)
	if !ok {
		return nil, false
	}
	return nexts, true
}

func (gb *GoSdApplicationBlock) Skipped(_ *store.VariableStore) []string {
	return nil
}

func (gb *GoSdApplicationBlock) GetState() interface{} {
	return gb.State
}

func (gb *GoSdApplicationBlock) Update(_ context.Context, _ *script.BlockUpdateData) (interface{}, error) {
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

func createGoSdApplicationBlock(name string, ef *entity.EriusFunc) (*GoSdApplicationBlock, error) {
	log := logger.CreateLogger(nil)
	log.WithField("params", ef.Params).Info("sd_application parameters")

	b := &GoSdApplicationBlock{
		Name:    name,
		Title:   ef.Title,
		Input:   map[string]string{},
		Output:  map[string]string{},
		Sockets: entity.ConvertSocket(ef.Sockets),
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

	return b, nil
}
