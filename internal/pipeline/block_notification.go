package pipeline

import (
	"context"
	"encoding/json"
	"strings"

	"go.opencensus.io/trace"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type NotificationData struct {
	People  []string `json:"people"`
	Subject string   `json:"subject"`
	Text    string   `json:"text"`
}

type GoNotificationBlock struct {
	Name   string
	Title  string
	Input  map[string]string
	Output map[string]string
	Nexts  map[string][]string
	State  *NotificationData

	Pipeline *ExecutablePipeline
}

func (gb *GoNotificationBlock) GetStatus() Status {
	return StatusFinished
}

func (gb *GoNotificationBlock) GetTaskHumanStatus() TaskHumanStatus {
	return ""
}

func (gb *GoNotificationBlock) GetType() string {
	return BlockGoNotificationID
}

func (gb *GoNotificationBlock) Inputs() map[string]string {
	return gb.Input
}

func (gb *GoNotificationBlock) Outputs() map[string]string {
	return gb.Output
}

func (gb *GoNotificationBlock) IsScenario() bool {
	return false
}

func (gb *GoNotificationBlock) DebugRun(ctx context.Context, _ *stepCtx, _ *store.VariableStore) (err error) {
	ctx, s := trace.StartSpan(ctx, "run_go_notification_block")
	defer s.End()

	emails := make([]string, 0, len(gb.State.People))
	for _, person := range gb.State.People {
		if strings.Contains(person, "@") {
			emails = append(emails, person)
			continue
		}
		email, err := gb.Pipeline.People.GetUserEmail(ctx, person)
		if err != nil {
			return err
		}
		emails = append(emails, email)
	}

	return gb.Pipeline.Sender.SendNotification(ctx, emails, mail.Template{
		Subject:   gb.State.Subject,
		Text:      gb.State.Text,
		Variables: nil,
	})
}

func (gb *GoNotificationBlock) Next(_ *store.VariableStore) ([]string, bool) {
	nexts, ok := gb.Nexts[DefaultSocket]
	if !ok {
		return nil, false
	}
	return nexts, true
}

func (gb *GoNotificationBlock) GetState() interface{} {
	return gb.State
}

func (gb *GoNotificationBlock) Update(_ context.Context, _ *script.BlockUpdateData) (interface{}, error) {
	return nil, nil
}

func (gb *GoNotificationBlock) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockGoNotificationID,
		BlockType: script.TypeGo,
		Title:     BlockGoNotificationTitle,
		Inputs:    nil,
		Outputs:   nil,
		Params: &script.FunctionParams{
			Type: BlockGoNotificationID,
			Params: &script.NotificationParams{
				People:  []string{},
				Subject: "",
				Text:    "",
			},
		},
		Sockets: []string{DefaultSocket},
	}
}

// nolint:dupl // another block
func createGoNotificationBlock(name string, ef *entity.EriusFunc, pipeline *ExecutablePipeline) (*GoNotificationBlock, error) {
	b := &GoNotificationBlock{
		Name:   name,
		Title:  ef.Title,
		Input:  map[string]string{},
		Output: map[string]string{},
		Nexts:  ef.Next,

		Pipeline: pipeline,
	}

	for _, v := range ef.Input {
		b.Input[v.Name] = v.Global
	}

	for _, v := range ef.Output {
		b.Output[v.Name] = v.Global
	}

	var params script.NotificationParams
	err := json.Unmarshal(ef.Params, &params)
	if err != nil {
		return nil, errors.Wrap(err, "can not get notification parameters")
	}

	if err = params.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid notification parameters")
	}

	b.State = &NotificationData{
		People:  params.People,
		Text:    params.Text,
		Subject: params.Subject,
	}

	return b, nil
}
