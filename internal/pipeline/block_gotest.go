package pipeline

import (
	"context"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type GoTestBlock struct {
	Name      string
	ShortName string
	Title     string
	Input     map[string]string
	Output    map[string]string
	Sockets   []script.Socket

	RunContext *BlockRunContext

	expectedEvents      map[string]struct{}
	happenedEvents      []entity.NodeEvent
	happenedKafkaEvents []entity.NodeKafkaEvent
}

func (gb *GoTestBlock) CurrentExecutorData() CurrentExecutorData {
	return CurrentExecutorData{}
}

func (gb *GoTestBlock) GetNewEvents() []entity.NodeEvent {
	return gb.happenedEvents
}

func (gb *GoTestBlock) GetNewKafkaEvents() []entity.NodeKafkaEvent {
	return gb.happenedKafkaEvents
}

func (gb *GoTestBlock) Members() []Member {
	return nil
}

func (gb *GoTestBlock) Deadlines(_ context.Context) ([]Deadline, error) {
	return []Deadline{}, nil
}

func (gb *GoTestBlock) UpdateManual() bool {
	return false
}

func (gb *GoTestBlock) GetStatus() Status {
	return StatusFinished
}

func (gb *GoTestBlock) GetTaskHumanStatus() (status TaskHumanStatus, comment, action string) {
	return "", "", ""
}

func (gb *GoTestBlock) GetType() string {
	return BlockGoTestID
}

func (gb *GoTestBlock) Inputs() map[string]string {
	return gb.Input
}

func (gb *GoTestBlock) Outputs() map[string]string {
	return gb.Output
}

func (gb *GoTestBlock) IsScenario() bool {
	return false
}

func (gb *GoTestBlock) BlockAttachments() (ids []string) {
	return ids
}

type stepCtx struct{}

// nolint:dupl // not dupl?
func (gb *GoTestBlock) DebugRun(ctx context.Context, _ *stepCtx, runCtx *store.VariableStore) error {
	_, s := trace.StartSpan(ctx, "run_go_block")
	defer s.End()

	runCtx.AddStep(gb.Name)

	values := make(map[string]interface{})

	for ikey, gkey := range gb.Input {
		val, ok := runCtx.GetValue(gkey) // if no value - empty value
		if ok {
			values[ikey] = val
		}
	}

	for ikey, gkey := range gb.Output {
		val, ok := values[ikey]
		if ok {
			runCtx.SetValue(gkey, val)
		}
	}

	return nil
}

func (gb *GoTestBlock) Next(_ *store.VariableStore) ([]string, bool) {
	nexts, ok := script.GetNexts(gb.Sockets, DefaultSocketID)
	if !ok {
		return nil, false
	}

	return nexts, true
}

func (gb *GoTestBlock) Skipped(_ *store.VariableStore) []string {
	return nil
}

func (gb *GoTestBlock) GetState() interface{} {
	return nil
}

func (gb *GoTestBlock) Update(ctx context.Context) (interface{}, error) {
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

//nolint:unparam // reEntry всегда false, но так надо
func createGoTestBlock(
	ctx context.Context,
	name string,
	ef *entity.EriusFunc,
	runCtx *BlockRunContext,
	expectedEvents map[string]struct{},
) (b *GoTestBlock, reEntry bool, err error) {
	b = &GoTestBlock{
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

func (gb *GoTestBlock) UpdateStateUsingOutput(ctx context.Context, data []byte) (state map[string]interface{}, err error) {
	return state, nil
}

func (gb *GoTestBlock) UpdateOutputUsingState(ctx context.Context) (output map[string]interface{}, err error) {
	return output, nil
}
