package scenario

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/idgen"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/props"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/state"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/user"
	"time"
)

type CreateScenarioDto struct {
	Name        string
	Nodes       []CreateScenarioNodeDto
	Transitions []CreateScenarioTransitionDto
	Input       []CreateScenarioProps
	Output      []CreateScenarioProps
}

type CreateScenarioNodeDto struct {
	Id     string
	Name   string
	Type   string
	Input  []CreateScenarioProps
	Output []CreateScenarioProps
}

type CreateScenarioProps struct {
	Key   string
	Type  string
	Value string
	Scope string
}

type CreateScenarioTransitionDto struct {
	FromPort string
	From     string
	To       string
}

type NodeId = string

func (d *CreateScenarioDto) ToScenario(idDict map[NodeId]NodeId) (*Scenario, error) {

	nodes := make([]state.Node, 0)
	for _, node := range d.Nodes {
		nodeId := idDict[node.Id]

		nodeProps := make(map[string]props.Prop)
		for _, p := range node.Input {
			prop, err := props.Repository.New(props.PropType(p.Type), p.Value)
			if err != nil {
				return nil, err
			}
			nodeProps[p.Key] = prop
		}

		nodes = append(nodes, state.NewStateNode(nodeId, node.Type, nodeProps))
	}

	transitions := make([]state.Transition, 0)

	for _, t := range d.Transitions {
		transitions = append(transitions, state.NewTransition(idgen.NewUUID(), idDict[t.From], t.FromPort, idDict[t.To]))
	}

	s, err := state.NewState(nodes, transitions)
	if err != nil {
		return nil, err
	}

	return &Scenario{
		Id:        idgen.NewUUID(),
		VersionId: idgen.NewUUID(),
		Name:      d.Name,
		State:     s,
	}, nil
}

func (s *Service) CreateScenario(ctx context.Context, createScenarioDto *entity.EriusScenarioV2) (*entity.EriusScenarioV2, error) {

	now := time.Now()
	createScenarioDto.CreatedAt = &now
	createScenarioDto.VersionID = uuid.New()
	createScenarioDto.ID = uuid.New()

	var b []byte
	b, err := json.Marshal(createScenarioDto)
	if err != nil {
		return nil, err
	}

	ctx = user.SetUserInfoToCtx(ctx, &sso.UserInfo{
		Username: "svkorot2",
	})

	u, err := user.GetUserInfoFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	isScenarioNameExist, err := s.store.IsScenarioNameExist(ctx, createScenarioDto.Name)
	if err != nil {
		return nil, err
	}

	if isScenarioNameExist {
		return nil, errors.New("name is already exist: " + createScenarioDto.Name)
	}

	err = s.store.CreatePipeline(ctx, createScenarioDto, u.Username, b)
	if err != nil {
		return nil, err
	}

	return s.GetScenario(ctx, createScenarioDto.VersionID.String())
}

func (t *Scenario) Validate() error {
	return nil
}
