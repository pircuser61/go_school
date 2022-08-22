package pipeline

import (
	"context"
	"encoding/json"
	"log"

	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type NotificationData struct {
	People  []string `json:"people"`
	Emails  []string `json:"emails"`
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

func (gb *GoNotificationBlock) compileText(ctx context.Context) (string, error) {
	author, err := gb.Pipeline.People.GetUser(ctx, gb.Pipeline.Initiator)
	if err != nil {

	}
	typedAuthor, err := author.ToSSOUserTyped()
	if err != nil {

	}
	text := mail.MakeBodyHeader(typedAuthor.Username, typedAuthor.Attributes.FullName,
		gb.Pipeline.Sender.GetApplicationLink(gb.Pipeline.WorkNumber), gb.State.Text)
	keys, err := gb.Pipeline.ServiceDesc.GetSchemaFieldsByApplication(ctx, gb.Pipeline.WorkNumber)
	if err != nil {

	}
	descr := mail.MakeDescription(data, keys)
	text = mail.WrapDescription(text, descr, attachments)
	text = mail.AddStyles(text)
	return text, nil
}

func (gb *GoNotificationBlock) DebugRun(ctx context.Context, _ *stepCtx, _ *store.VariableStore) (err error) {
	ctx, s := trace.StartSpan(ctx, "run_go_notification_block")
	defer s.End()

	emails := make([]string, 0, len(gb.State.People)+len(gb.State.Emails))
	for _, person := range gb.State.People {
		email, err := gb.Pipeline.People.GetUserEmail(ctx, person)
		if err != nil {
			log.Println("can't get email of user", person)
			continue
		}
		emails = append(emails, email)
	}
	emails = append(emails, gb.State.Emails...)

	if len(emails) == 0 {
		return errors.New("can't find any working emails from logins")
	}

	text, err := gb.compileText(ctx)
	if err != nil {

	}
	return gb.Pipeline.Sender.SendNotification(ctx, emails, mail.Template{
		Subject:   gb.State.Subject,
		Text:      text,
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

func (gb *GoNotificationBlock) Skipped(_ *store.VariableStore) []string {
	return nil
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
				Emails:  []string{},
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
		Emails:  params.Emails,
		Text:    params.Text,
		Subject: params.Subject,
	}

	return b, nil
}
