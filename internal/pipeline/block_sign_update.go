package pipeline

import (
	c "context"
	"encoding/json"
	"errors"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

type SignSignatureParams struct {
	Decision SignDecision `json:"decision"`
	Comment  string       `json:"comment,omitempty"`
}

func (gb *GoSignBlock) handleSignature() error {
	var updateParams SignSignatureParams

	err := json.Unmarshal(gb.RunContext.UpdateData.Parameters, &updateParams)
	if err != nil {
		return errors.New("can't assert provided update data")
	}

	if errSet := gb.State.SetDecision(gb.RunContext.UpdateData.ByLogin, &updateParams); errSet != nil {
		return errSet
	}

	if gb.State.Decision != nil {
		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputSigner], &gb.State.ActualSigner)
		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputSignDecision], &gb.State.Decision)
		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputSignComment], &gb.State.Comment)
	}

	return nil
}

func (gb *GoSignBlock) Update(_ c.Context) (interface{}, error) {
	data := gb.RunContext.UpdateData
	if data == nil {
		return nil, errors.New("empty data")
	}

	//nolint:gocritic //for future actions
	switch data.Action {
	case string(entity.TaskUpdateActionSign):
		if errUpdate := gb.handleSignature(); errUpdate != nil {
			return nil, errUpdate
		}
	}

	var stateBytes []byte
	stateBytes, err := json.Marshal(gb.State)
	if err != nil {
		return nil, err
	}

	gb.RunContext.VarStore.ReplaceState(gb.Name, stateBytes)

	return nil, nil
}
