package pipeline

import (
	c "context"
	"encoding/json"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

// nolint:dupl // another block
func createGoApproverBlock(ctx c.Context, name string, ef *entity.EriusFunc, ep *ExecutablePipeline) (*GoApproverBlock, error) {
	b := &GoApproverBlock{
		Name:   name,
		Title:  ef.Title,
		Input:  map[string]string{},
		Output: map[string]string{},
		Nexts:  ef.Next,

		Pipeline: ep,
	}

	for _, v := range ef.Input {
		b.Input[v.Name] = v.Global
	}

	// TODO: check existence of keyApproverDecision in Output

	for _, v := range ef.Output {
		b.Output[v.Name] = v.Global
	}

	var params script.ApproverParams
	err := json.Unmarshal(ef.Params, &params)
	if err != nil {
		return nil, errors.Wrap(err, "can not get approver parameters")
	}

	if err = params.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid approver parameters")
	}

	approvers := map[string]struct{}{
		params.Approver: {},
	}

	approversGroupName := ""

	if params.Type == script.ApproverTypeGroup {
		approversGroup, errGroup := ep.ServiceDesc.GetApproversGroup(ctx, params.ApproversGroupID)
		if errGroup != nil {
			return nil, errors.Wrap(errGroup, "can`t get approvers group with id: "+params.ApproversGroupID)
		}

		if len(approversGroup.People) == 0 {
			return nil, errors.Wrap(errGroup, "zero approvers in group: "+params.ApproversGroupID)
		}

		approversGroupName = approversGroup.GroupName

		approvers = make(map[string]struct{})
		for i := range approversGroup.People {
			approvers[approversGroup.People[i].Login] = struct{}{}
		}
	}

	b.State = &ApproverData{
		Type:               params.Type,
		Approvers:          approvers,
		SLA:                params.SLA,
		AutoAction:         params.AutoAction,
		LeftToNotify:       approvers,
		IsEditable:         params.IsEditable,
		RepeatPrevDecision: params.RepeatPrevDecision,
		ApproversGroupID:   params.ApproversGroupID,
		ApproversGroupName: approversGroupName,
		ApproverLog:        make([]ApproverLogEntry, 0),
	}

	if b.State.ApprovementRule == "" {
		b.State.ApprovementRule = AnyOfApprovementRequired
	}

	return b, nil
}
