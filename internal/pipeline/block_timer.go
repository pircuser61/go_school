package pipeline

import (
	c "context"
	"encoding/json"
	"gitlab.services.mts.ru/abp/myosotis/logger"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/net/context"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type TimerData struct {
	Duration time.Duration
	Started  bool
	Expired  bool
}

type TimerBlock struct {
	Name    string
	Title   string
	Input   map[string]string
	Output  map[string]string
	Sockets []script.Socket
	State   *TimerData

	RunContext *BlockRunContext
}

type TimerParams struct {
	Duration string `json:"duration"`
}

func (gb *TimerBlock) Members() []Member {
	return nil
}

func (gb *TimerBlock) Deadlines(_ context.Context) ([]Deadline, error) {
	return []Deadline{}, nil
}

func (gb *TimerBlock) GetStatus() Status {
	if gb.State.Expired {
		return StatusFinished
	} else {
		return StatusIdle
	}
}

func (gb *TimerBlock) GetTaskHumanStatus() TaskHumanStatus {
	if gb.State.Expired {
		return StatusDone
	} else {
		return StatusExecution
		// может лучше? return StatusWait
	}
}

func (gb *TimerBlock) Next(_ *store.VariableStore) ([]string, bool) {
	nexts, ok := script.GetNexts(gb.Sockets, DefaultSocketID)
	if !ok {
		return nil, false
	}
	return nexts, true
}

func (gb *TimerBlock) GetState() interface{} {
	return gb.State
}

//nolint:gocyclo //its ok here
func (gb *TimerBlock) Update(ctx c.Context) (interface{}, error) {
	if gb.State.Expired {
		return nil, errors.New("timer has already expired")
	}

	if gb.State.Started {
		gb.State.Expired = true
	} else {
		go gb.startTimer(ctx)
		gb.State.Started = true
	}

	var stateBytes []byte
	stateBytes, err := json.Marshal(gb.State)
	if err != nil {
		return nil, err
	}

	gb.RunContext.VarStore.ReplaceState(gb.Name, stateBytes)

	return nil, nil
}

func (gb *TimerBlock) startTimer(ctx c.Context) {
	log := logger.GetLogger(ctx)
	log.Info("timer started")
	time.Sleep(gb.State.Duration)
	log.Info("timer is up")
}

func (gb *TimerBlock) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockTimerID,
		BlockType: script.TypeGo,
		Title:     BlockTimerTitle,
		Inputs:    nil,
		Outputs:   nil,
		Params: &script.FunctionParams{
			Type: BlockTimerID,
			Params: TimerParams{
				Duration: "0s",
			},
		},
		Sockets: []script.Socket{script.DefaultSocket},
	}
}

func (gb *TimerBlock) UpdateManual() bool {
	return false
}

// nolint:dupl // another block
func createTimerBlock(name string, ef *entity.EriusFunc, runCtx *BlockRunContext) (*TimerBlock, bool, error) {
	b := &TimerBlock{
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

	rawState, blockExists := runCtx.VarStore.State[name]
	reEntry := blockExists && runCtx.UpdateData == nil
	if blockExists {
		if err := b.loadState(rawState); err != nil {
			return nil, false, err
		}
	} else {
		if err := b.createState(ef); err != nil {
			return nil, false, err
		}
		b.RunContext.VarStore.AddStep(b.Name)
	}

	return b, reEntry, nil
}

func (gb *TimerBlock) loadState(raw json.RawMessage) error {
	return json.Unmarshal(raw, &gb.State)
}

//nolint:dupl,gocyclo //its not duplicate
func (gb *TimerBlock) createState(ef *entity.EriusFunc) error {
	var params TimerParams
	err := json.Unmarshal(ef.Params, &params)
	if err != nil {
		return errors.Wrap(err, "can not get timer parameters")
	}

	var duration time.Duration
	duration, err = time.ParseDuration(params.Duration)
	if err != nil {
		return errors.Wrap(err, "can not parse timer duration")
	}

	if duration <= 0 {
		return errors.New("delay time is not set for the timer")
	}

	gb.State = &TimerData{Duration: duration}

	return nil
}
