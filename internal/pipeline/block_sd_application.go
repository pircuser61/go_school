package pipeline

import (
	"context"
	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
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
	BlueprintID string `json:"blueprint_id"`
}

type SdApplicationData struct {
	BlueprintID     string                 `json:"blueprint_id"`
	Description     string                 `json:"description"`
	ApplicationBody map[string]interface{} `json:"application_body"`
}

type GoSdApplicationBlock struct {
	Name     string
	Title    string
	Input    map[string]string
	Output   map[string]string
	NextStep string
	State    *ApplicationData

	Storage db.Database
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

func (gb *GoSdApplicationBlock) Run(ctx context.Context, runCtx *store.VariableStore) error {
	return gb.DebugRun(ctx, runCtx)
}

func (gb *GoSdApplicationBlock) DebugRun(ctx context.Context, runCtx *store.VariableStore) (err error) {
	_, s := trace.StartSpan(ctx, "run_go_sd_block")
	defer s.End()

	runCtx.AddStep(gb.Name)

	data := ctx.Value(SdApplicationDataCtx{})
	if data == nil {
		return errors.New("can't find application data in context")
	}

	appData, ok := data.(SdApplicationData)
	if !ok {
		return errors.New("invalid application data in context")
	}

	runCtx.SetValue(gb.Output[keyOutputBlueprintID], appData.BlueprintID)
	runCtx.SetValue(gb.Output[keyOutputSdApplicationDesc], appData.Description)
	runCtx.SetValue(gb.Output[keyOutputSdApplication], appData.ApplicationBody)

	return err
}

func (gb *GoSdApplicationBlock) Next(_ *store.VariableStore) (string, bool) {
	return gb.NextStep, true
}

func (gb *GoSdApplicationBlock) NextSteps() []string {
	nextSteps := []string{gb.NextStep}

	return nextSteps
}

func (gb *GoSdApplicationBlock) GetState() interface{} {
	return gb.State
}

func (gb *GoSdApplicationBlock) Update(_ context.Context, _ interface{}) (interface{}, error) {
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
				Comment: "application blueprint id",
			},
			{
				Name:    keyOutputSdApplicationDesc,
				Type:    "string",
				Comment: "application description",
			},
			{
				Name:    keyOutputSdApplication,
				Type:    "string",
				Comment: "application body",
			},
		},
		Params: &script.FunctionParams{
			Type: BlockGoSdApplicationID,
			Params: &script.SdApplicationParams{
				BlueprintID: "",
			},
		},
		NextFuncs: []string{script.Next},
	}
}

func createGoSdApplicationBlock(name string, ef *entity.EriusFunc, storage db.Database) (*GoSdApplicationBlock, error) {
	b := &GoSdApplicationBlock{
		Storage: storage,

		Name:     name,
		Title:    ef.Title,
		Input:    map[string]string{},
		Output:   map[string]string{},
		NextStep: ef.Next,
	}

	for _, v := range ef.Input {
		b.Input[v.Name] = v.Global
	}

	for _, v := range ef.Output {
		b.Output[v.Name] = v.Global
	}

	params, ok := ef.Params.(*script.SdApplicationParams)
	if !ok || params == nil {
		return nil, errors.New("can not get sd_application parameters")
	}

	if err := params.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid sd_application parameters")
	}

	b.State = &ApplicationData{
		BlueprintID: params.BlueprintID,
	}

	return b, nil
}
