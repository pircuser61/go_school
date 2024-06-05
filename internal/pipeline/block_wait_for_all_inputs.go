package pipeline

import (
	"context"
	"encoding/json"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type SyncData struct {
	IncomingBlockIds []string `json:"incoming_block_ids"`
	Done             bool     `json:"done"`
}

type GoWaitForAllInputsBlock struct {
	Name      string
	ShortName string
	Title     string
	Input     map[string]string
	Output    map[string]string
	Sockets   []script.Socket

	State *SyncData

	RunContext *BlockRunContext

	expectedEvents      map[string]struct{}
	happenedEvents      []entity.NodeEvent
	happenedKafkaEvents []entity.NodeKafkaEvent
}

func (gb *GoWaitForAllInputsBlock) CurrentExecutorData() CurrentExecutorData {
	return CurrentExecutorData{}
}

func (gb *GoWaitForAllInputsBlock) GetNewEvents() []entity.NodeEvent {
	return gb.happenedEvents
}

func (gb *GoWaitForAllInputsBlock) GetNewKafkaEvents() []entity.NodeKafkaEvent {
	return gb.happenedKafkaEvents
}

func (gb *GoWaitForAllInputsBlock) Members() []Member {
	return nil
}

func (gb *GoWaitForAllInputsBlock) Deadlines(_ context.Context) ([]Deadline, error) {
	return []Deadline{}, nil
}

func (gb *GoWaitForAllInputsBlock) UpdateManual() bool {
	return false
}

func (gb *GoWaitForAllInputsBlock) GetStatus() Status {
	if gb.State.Done {
		return StatusFinished
	}

	return StatusRunning
}

func (gb *GoWaitForAllInputsBlock) GetTaskHumanStatus() (status TaskHumanStatus, comment, action string) {
	if gb.State.Done {
		return StatusDone, "", ""
	}

	return StatusExecution, "", ""
}

func (gb *GoWaitForAllInputsBlock) Next(_ *store.VariableStore) ([]string, bool) {
	nexts, ok := script.GetNexts(gb.Sockets, DefaultSocketID)
	if !ok {
		return nil, false
	}

	return nexts, true
}

func (gb *GoWaitForAllInputsBlock) GetState() interface{} {
	return gb.State
}

func (gb *GoWaitForAllInputsBlock) Update(ctx context.Context) (interface{}, error) {
	executed, err := gb.RunContext.Services.Storage.ParallelIsFinished(ctx, gb.RunContext.WorkNumber, gb.Name)
	if err != nil {
		return nil, err
	}

	if !executed {
		return nil, nil
	}

	variableStorage, err := gb.RunContext.Services.Storage.GetMergedVariableStorage(ctx, gb.RunContext.TaskID,
		gb.State.IncomingBlockIds)
	if err != nil {
		return nil, err
	}

	gb.RunContext.VarStore = variableStorage
	gb.State.Done = executed

	state, stateErr := json.Marshal(gb.GetState())
	if stateErr != nil {
		return nil, stateErr
	}

	gb.RunContext.VarStore.ReplaceState(gb.Name, state)

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

func (gb *GoWaitForAllInputsBlock) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockWaitForAllInputsID,
		BlockType: script.TypeGo,
		Title:     BlockGoWaitForAllInputsTitle,
		Inputs:    nil,
		Outputs:   nil,
		Sockets:   []script.Socket{script.DefaultSocket},
	}
}

func (gb *GoWaitForAllInputsBlock) BlockAttachments() (ids []string) {
	return ids
}

//nolint:unparam // reEntry always false // когда-нибудь обязательно дорастёт до true
func createGoWaitForAllInputsBlock(
	ctx context.Context,
	name string,
	ef *entity.EriusFunc,
	runCtx *BlockRunContext,
	expectedEvents map[string]struct{},
) (block *GoWaitForAllInputsBlock, reEntry bool, err error) {
	b := &GoWaitForAllInputsBlock{
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
		//nolint:gocritic //не в моих силах поменять коллекцию на поинтеры
		for propertyName, v := range ef.Output.Properties {
			if v.Global == "" {
				continue
			}

			b.Output[propertyName] = v.Global
		}
	}

	rawState, ok := runCtx.VarStore.State[name]
	if ok {
		if err := b.loadState(rawState); err != nil {
			return nil, reEntry, err
		}

		b.State.Done = false
	} else {
		err := b.createExpectedEvents(ctx, runCtx, name, ef)
		if err != nil {
			return nil, reEntry, err
		}
	}

	return b, reEntry, nil
}

//nolint:dupl //another block
func (gb *GoWaitForAllInputsBlock) createExpectedEvents(
	ctx context.Context,
	runCtx *BlockRunContext,
	name string,
	ef *entity.EriusFunc,
) error {
	if err := gb.createState(ctx); err != nil {
		return err
	}

	gb.RunContext.VarStore.AddStep(gb.Name)

	if _, ok := gb.expectedEvents[eventStart]; ok {
		status, _, _ := gb.GetTaskHumanStatus()

		event, err := runCtx.MakeNodeStartEvent(
			ctx,
			MakeNodeStartEventArgs{
				NodeName:      name,
				NodeShortName: ef.ShortTitle,
				HumanStatus:   status,
				NodeStatus:    gb.GetStatus(),
			},
		)
		if err != nil {
			return err
		}

		gb.happenedEvents = append(gb.happenedEvents, event)
	}

	return nil
}

func (gb *GoWaitForAllInputsBlock) loadState(raw json.RawMessage) error {
	return json.Unmarshal(raw, &gb.State)
}

func (gb *GoWaitForAllInputsBlock) createState(ctx context.Context) error {
	steps, err := gb.RunContext.Services.Storage.GetTaskStepsToWait(ctx, gb.RunContext.WorkNumber, gb.Name)
	if err != nil {
		return err
	}

	gb.State = &SyncData{IncomingBlockIds: steps}

	return nil
}

func (gb *GoWaitForAllInputsBlock) UpdateStateUsingOutput(context.Context, []byte) (state map[string]interface{}, err error) {
	return nil, nil
}

func (gb *GoWaitForAllInputsBlock) UpdateOutputUsingState(context.Context) (output map[string]interface{}, err error) {
	return nil, nil
}
