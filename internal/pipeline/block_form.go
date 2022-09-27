package pipeline

import (
	c "context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"

	"go.opencensus.io/trace"
)

const (
	keyOutputExecutor = "executor"
	keyOutputFormBody = "form_body"
)

type FormData struct {
	FormId         string                 `json:"form_id"`
	Type           script.ApproverType    `json:"type"`
	Executors      map[string]struct{}    `json:"executors"`
	FormBody       map[string]interface{} `json:"form_body"`
	IsFilled       bool                   `json:"is_filled"`
	ActualExecutor *string                `json:"actual_executor,omitempty"`

	SLA int `json:"sla"`

	DidSLANotification bool `json:"did_sla_notification"`

	LeftToNotify map[string]struct{} `json:"left_to_notify"`
}

type GoFormBlock struct {
	Name    string
	Title   string
	Input   map[string]string
	Output  map[string]string
	Sockets []script.Socket
	State   *FormData

	Pipeline *ExecutablePipeline
}

func (gb *GoFormBlock) GetStatus() Status {
	if gb.State != nil && gb.State.IsFilled {
		return StatusFinished
	}

	return StatusRunning
}

func (gb *GoFormBlock) GetTaskHumanStatus() TaskHumanStatus {
	if gb.State != nil && gb.State.IsFilled {
		return StatusDone
	}

	return StatusExecution
}

func (gb *GoFormBlock) GetType() string {
	return BlockGoFormID
}

func (gb *GoFormBlock) Inputs() map[string]string {
	return gb.Input
}

func (gb *GoFormBlock) Outputs() map[string]string {
	return gb.Output
}

func (gb *GoFormBlock) IsScenario() bool {
	return false
}

//nolint:gocyclo //ok
func (gb *GoFormBlock) DebugRun(ctx c.Context, stepCtx *stepCtx, runCtx *store.VariableStore) (err error) {
	ctx, s := trace.StartSpan(ctx, "run_go_form_block")
	defer s.End()

	l := logger.GetLogger(ctx)

	val, isOk := runCtx.GetValue(getWorkIdKey(gb.Name))
	if !isOk {
		return errors.New("can't get work id from variable store")
	}

	id, isOk := val.(uuid.UUID)
	if !isOk {
		return errors.New("can't assert type of work id")
	}

	// check state from database
	var step *entity.Step
	step, err = gb.Pipeline.Storage.GetTaskStepById(ctx, id)
	if err != nil {
		return err
	} else if step == nil {
		l.Error(err)
		return nil
	}

	// get state from step.State
	data, ok := step.State[gb.Name]
	if !ok {
		return fmt.Errorf("key %s is not found in state of go-form-block", gb.Name)
	}

	var state FormData
	err = json.Unmarshal(data, &state)
	if err != nil {
		return errors.Wrap(err, "invalid format of go-form-block state")
	}

	gb.State = &state

	// nolint:dupl // not dupl?
	if gb.State.IsFilled {
		var actualExecutor string

		if state.ActualExecutor != nil {
			actualExecutor = *state.ActualExecutor
		}

		runCtx.SetValue(gb.Output[keyOutputExecutor], actualExecutor)
		runCtx.SetValue(gb.Output[keyOutputFormBody], gb.State.FormBody)

		var stateBytes []byte
		stateBytes, err = json.Marshal(gb.State)
		if err != nil {
			return err
		}

		runCtx.ReplaceState(gb.Name, stateBytes)
	}

	return nil
}

func (gb *GoFormBlock) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockGoApproverID,
		BlockType: script.TypeGo,
		Title:     gb.Title,
		Inputs:    nil,
		Outputs: []script.FunctionValueModel{
			{
				Name:    keyOutputApprover,
				Type:    "string",
				Comment: "approver login which made a decision",
			},
			{
				Name:    keyOutputDecision,
				Type:    "string",
				Comment: "block decision",
			},
			{
				Name:    keyOutputComment,
				Type:    "string",
				Comment: "approver comment",
			},
		},
		Params: &script.FunctionParams{
			Type: BlockGoApproverID,
			Params: &script.ApproverParams{
				Approver:           "",
				Type:               "",
				SLA:                0,
				IsEditable:         false,
				RepeatPrevDecision: false,
				ApproversGroupID:   "",
				ApproversGroupName: "",
			},
		},
		Sockets: []script.Socket{
			script.ApprovedSocket,
			script.RejectedSocket,
			script.EditAppSocket,
			script.RequestAddInfoSocket,
		},
	}
}
