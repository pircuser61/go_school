package scenario

import (
	"github.com/google/uuid"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/state"
)

type Scenario struct {
	Id        string
	VersionId string
	Name      string
	State     state.State
	Status    int // 1 - Draft, 2 - Approved, 3 - Deleted, 4 - Rejected, 5 - On Approve
	HasDraft  bool
	Input     []CreateScenarioProps
	Output    []CreateScenarioProps
}

func (s *Scenario) ToModelA() (*entity.EriusScenarioV2, error) {

	id, err := uuid.Parse(s.Id)
	if err != nil {
		return nil, err
	}

	versionID, err := uuid.Parse(s.VersionId)
	if err != nil {
		return nil, err
	}

	return &entity.EriusScenarioV2{
		ID:        id,
		VersionID: versionID,
		Status:    s.Status,
		HasDraft:  s.HasDraft,
		Name:      s.Name,
		Input:     nil,
		Output:    nil,
		Pipeline: struct {
			Entrypoint string                        `json:"entrypoint"`
			Blocks     map[string]entity.EriusFuncV2 `json:"blocks"`
		}{},
		CreatedAt:       nil,
		ApprovedAt:      nil,
		Author:          "",
		Tags:            nil,
		Comment:         "",
		CommentRejected: "",
	}, nil
}
