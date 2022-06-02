package pipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

const (
	keyApproverDecision = "approver"
)

type ApproverType string

func (a ApproverType) String() string {
	return string(a)
}

const (
	ApproverTypeUser       ApproverType = "user"
	ApproverTypeGroup      ApproverType = "group"
	ApproverTypeSupervisor ApproverType = "supervisor"
)

type ApproverDecision string

func (a ApproverDecision) String() string {
	return string(a)
}

const (
	ApproverDecisionApproved ApproverDecision = "approved"
	ApproverDecisionRejected ApproverDecision = "rejected"
)

type Approver struct {
	Decision *ApproverDecision `json:"decision,omitempty"`
	Comment  *string           `json:"comment,omitempty"`
}

type ApproverData struct {
	Type           ApproverType        `json:"type"`
	Approvers      map[string]struct{} `json:"approvers"`
	Decision       *ApproverDecision   `json:"decision,omitempty"`
	Comment        *string             `json:"comment,omitempty"`
	ActualApprover *string             `json:"actual_approver,omitempty"`
}

func (a *ApproverData) GetDecision() *ApproverDecision {
	return a.Decision
}

func (a *ApproverData) SetDecision(login string, decision ApproverDecision, comment string) error {
	_, ok := a.Approvers[login]
	if !ok {
		return fmt.Errorf("%s not found in approvers", login)
	}

	if a.Decision != nil {
		return errors.New("decision already set")
	}

	if decision != ApproverDecisionApproved && decision != ApproverDecisionRejected {
		return fmt.Errorf("unknown decision %s", decision.String())
	}

	a.Decision = &decision
	a.Comment = &comment
	a.ActualApprover = &login

	return nil
}

type ApproverResult struct {
	Login    string           `json:"login"`
	Decision ApproverDecision `json:"decision"`
	Comment  string           `json:"comment,omitempty"`
}

type GoApproverBlock struct {
	Name     string
	Title    string
	Input    map[string]string
	Output   map[string]string
	NextStep string

	Storage db.Database
}

func (gb *GoApproverBlock) GetType() string {
	return BlockGoApprover
}

func (gb *GoApproverBlock) Inputs() map[string]string {
	return gb.Input
}

func (gb *GoApproverBlock) Outputs() map[string]string {
	return gb.Output
}

func (gb *GoApproverBlock) IsScenario() bool {
	return false
}

func (gb *GoApproverBlock) Run(ctx context.Context, runCtx *store.VariableStore) error {
	return gb.DebugRun(ctx, runCtx)
}

func (gb *GoApproverBlock) DebugRun(ctx context.Context, runCtx *store.VariableStore) (err error) {
	_, s := trace.StartSpan(ctx, "run_go_approver_block")
	defer s.End()

	runCtx.AddStep(gb.Name)
	val, isOk := runCtx.GetValue(getWorkIdKey(gb.Name))
	if !isOk {
		return errors.New("can't get work id from variable store")
	}

	id, isOk := val.(uuid.UUID)
	if !isOk {
		return errors.New("can't assert type of work id")
	}

	var waitTime time.Duration
	var decision *ApproverDecision

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-time.After(waitTime):
			// update waiting time
			waitTime = time.Second * 10

			// check state from database
			var step *entity.Step
			step, err = gb.Storage.GetTaskStepById(ctx, id)
			if err != nil {
				return err
			} else if step == nil {
				// still waiting
				continue
			}

			// get state from step.Storage
			data, ok := step.Storage[gb.Name]
			if !ok {
				continue
			}

			state, ok := (data).(*ApproverData)
			if !ok {
				return errors.New("invalid format of go-approver-block state")
			} else if state == nil {
				continue
			}

			// check decision
			decision = state.GetDecision()
			if decision != nil {
				var actualApprover, comment string

				if state.ActualApprover != nil {
					actualApprover = *state.ActualApprover
				}

				if state.Comment != nil {
					comment = *state.Comment
				}

				runCtx.SetValue(gb.Output[keyApproverDecision], &ApproverResult{
					Login:    actualApprover,
					Decision: *decision,
					Comment:  comment,
				})

				return nil
			}
		}
	}
}

func (gb *GoApproverBlock) Next(_ *store.VariableStore) (string, bool) {
	return gb.NextStep, true
}

func (gb *GoApproverBlock) NextSteps() []string {
	nextSteps := []string{gb.NextStep}

	return nextSteps
}
