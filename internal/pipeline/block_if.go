package pipeline

import (
	"context"
	"encoding/json"

	//nolint:goimports //cant sort import to not trigger golint
	conditions_kit "gitlab.services.mts.ru/jocasta/conditions-kit"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type IF struct {
	Name          string
	ShortName     string
	Title         string
	Input         map[string]string
	Output        map[string]string
	FunctionName  string
	FunctionInput map[string]string
	Result        bool
	Sockets       []script.Socket
	State         *ConditionsData

	RunContext *BlockRunContext

	expectedEvents      map[string]struct{}
	happenedEvents      []entity.NodeEvent
	happenedKafkaEvents []entity.NodeKafkaEvent
}

func (gb *IF) CurrentExecutorData() CurrentExecutorData {
	return CurrentExecutorData{}
}

func (gb *IF) GetNewEvents() []entity.NodeEvent {
	return gb.happenedEvents
}

func (gb *IF) GetNewKafkaEvents() []entity.NodeKafkaEvent {
	return gb.happenedKafkaEvents
}

func (gb *IF) Members() []Member {
	return nil
}

func (gb *IF) Deadlines(_ context.Context) ([]Deadline, error) {
	return []Deadline{}, nil
}

type ConditionsData struct {
	Type            conditions_kit.ConditionType    `json:"type"`
	ConditionGroups []conditions_kit.ConditionGroup `json:"conditionGroups"`
	ChosenGroupID   string                          `json:"-"`
}

func (cd *ConditionsData) GetConditionGroups() []conditions_kit.ConditionGroup {
	return cd.ConditionGroups
}

func (gb *IF) UpdateManual() bool {
	return false
}

func (gb *IF) GetStatus() Status {
	return StatusFinished
}

func (gb *IF) GetTaskHumanStatus() (status TaskHumanStatus, comment, action string) {
	return "", "", ""
}

func (gb *IF) Next(_ *store.VariableStore) ([]string, bool) {
	if gb.State.ChosenGroupID == "" {
		nexts, ok := script.GetNexts(gb.Sockets, DefaultSocketID)
		if !ok {
			return nil, false
		}

		return nexts, true
	}

	nexts, ok := script.GetNexts(gb.Sockets, gb.State.ChosenGroupID)
	if !ok {
		return nil, false
	}

	return nexts, true
}

func (gb *IF) GetState() interface{} {
	return nil
}

func (gb *IF) Update(ctx context.Context) (interface{}, error) {
	var chosenGroup *conditions_kit.ConditionGroup

	if gb.State != nil {
		conditionGroups := gb.State.GetConditionGroups()

		variables, err := getVariables(gb.RunContext.VarStore)
		if err != nil {
			return nil, err
		}

		chosenGroup = conditions_kit.ProcessConditionGroups(conditionGroups, variables)
	}

	var chosenGroupID string

	if chosenGroup != nil {
		chosenGroupID = chosenGroup.Id
	} else {
		chosenGroupID = ""
	}

	gb.State.ChosenGroupID = chosenGroupID

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

func (gb *IF) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockGoIfID,
		BlockType: script.TypeGo,
		Title:     BlockGoIfTitle,
		Inputs:    nil,
		Outputs:   nil,
		Params: &script.FunctionParams{
			Type: BlockGoIfID,
			Params: &conditions_kit.ConditionParams{
				Type: "",
			},
		},
		Sockets: []script.Socket{script.DefaultSocket},
	}
}

func (gb *IF) BlockAttachments() (ids []string) {
	return ids
}

//nolint:unparam // its ok
func createGoIfBlock(ctx context.Context, name string, ef *entity.EriusFunc, runCtx *BlockRunContext,
	expectedEvents map[string]struct{},
) (block *IF, reEntry bool, err error) {
	b := &IF{
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

	b.State = &ConditionsData{
		ChosenGroupID: "",
	}

	if ef.Params != nil {
		var params conditions_kit.ConditionParams

		err = json.Unmarshal(ef.Params, &params)
		if err != nil {
			return nil, reEntry, err
		}

		err = params.Validate()
		if err != nil {
			return nil, reEntry, err
		}

		b.State.Type = params.Type
		b.State.ConditionGroups = params.ConditionGroups
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

func getVariables(runCtx *store.VariableStore) (result map[string]interface{}, err error) {
	variables, err := runCtx.GrabStorage()
	if err != nil {
		return nil, err
	}

	return variables, nil
}

func (gb *IF) UpdateStateUsingOutput(context.Context, []byte) (state map[string]interface{}, err error) {
	return nil, nil
}

func (gb *IF) UpdateOutputUsingState(context.Context) (output map[string]interface{}, err error) {
	return nil, nil
}
