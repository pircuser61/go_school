package pipeline

import (
	"context"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type GoStartBlock struct {
	Name       string
	ShortName  string
	Title      string
	Input      map[string]string
	Output     map[string]string
	Sockets    []script.Socket
	RunContext *BlockRunContext

	expectedEvents      map[string]struct{}
	happenedEvents      []entity.NodeEvent
	happenedKafkaEvents []entity.NodeKafkaEvent
}

func (gb *GoStartBlock) CurrentExecutorData() CurrentExecutorData {
	return CurrentExecutorData{}
}

func (gb *GoStartBlock) GetNewEvents() []entity.NodeEvent {
	return gb.happenedEvents
}

func (gb *GoStartBlock) GetNewKafkaEvents() []entity.NodeKafkaEvent {
	return gb.happenedKafkaEvents
}

func (gb *GoStartBlock) Members() []Member {
	return nil
}

func (gb *GoStartBlock) Deadlines(_ context.Context) ([]Deadline, error) {
	return []Deadline{}, nil
}

func (gb *GoStartBlock) UpdateManual() bool {
	return false
}

func (gb *GoStartBlock) GetStatus() Status {
	return StatusFinished
}

func (gb *GoStartBlock) GetTaskHumanStatus() (status TaskHumanStatus, comment, action string) {
	return StatusNew, "", ""
}

func (gb *GoStartBlock) Next(_ *store.VariableStore) ([]string, bool) {
	nexts, ok := script.GetNexts(gb.Sockets, DefaultSocketID)
	if !ok {
		return nil, false
	}

	return nexts, true
}

func (gb *GoStartBlock) GetState() interface{} {
	return nil
}

func (gb *GoStartBlock) Update(ctx context.Context) (interface{}, error) {
	data, err := gb.RunContext.Services.Storage.GetTaskRunContext(ctx, gb.RunContext.WorkNumber)
	if err != nil {
		return nil, errors.Wrap(err, "can't get task run context")
	}

	personData, err := gb.RunContext.Services.ServiceDesc.GetSsoPerson(ctx, gb.RunContext.Initiator)
	if err != nil {
		return nil, err
	}

	gb.RunContext.VarStore.SetValue(gb.Output[entity.KeyOutputWorkNumber], gb.RunContext.WorkNumber)
	gb.RunContext.VarStore.SetValue(gb.Output[entity.KeyOutputApplicationInitiator], personData)
	gb.RunContext.VarStore.SetValue(gb.Output[entity.KeyOutputApplicationBody], data.InitialApplication.ApplicationBody)

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

func (gb *GoStartBlock) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockGoStartID,
		BlockType: script.TypeGo,
		Title:     BlockGoStartTitle,
		Inputs:    nil,
		Outputs: &script.JSONSchema{
			Type: "object",
			Properties: script.JSONSchemaProperties{
				entity.KeyOutputWorkNumber: {
					Type:        "string",
					Description: "work number",
				},
				entity.KeyOutputApplicationInitiator: {
					Type:        "object",
					Description: "person object from sso",
					Format:      "SsoPerson",
					Properties:  people.GetSsoPersonSchemaProperties(),
				},
				entity.KeyOutputApplicationBody: {
					Type:       "object",
					Properties: script.JSONSchemaProperties{},
				},
			},
		},
		Sockets: []script.Socket{script.DefaultSocket},
	}
}

func (gb *GoStartBlock) BlockAttachments() (ids []string) {
	return ids
}

//nolint:dupl,unparam //its not duplicate
func createGoStartBlock(ctx context.Context, name string, ef *entity.EriusFunc, runCtx *BlockRunContext,
	expectedEvents map[string]struct{},
) (*GoStartBlock, bool, error) {
	b := &GoStartBlock{
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
			b.Output[propertyName] = v.Global
		}
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

	return b, false, nil
}

func (gb *GoStartBlock) UpdateStateUsingOutput(ctx context.Context, data []byte) (state map[string]interface{}, err error) {
	return nil, nil
}

func (gb *GoStartBlock) UpdateOutputUsingState(ctx context.Context) (output map[string]interface{}, err error) {
	return nil, nil
}
