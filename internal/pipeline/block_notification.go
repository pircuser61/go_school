package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/abp/mail/pkg/email"

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
	Name    string
	Title   string
	Input   map[string]string
	Output  map[string]string
	Sockets []script.Socket
	State   *NotificationData

	RunContext *BlockRunContext
}

func (gb *GoNotificationBlock) Members() []Member {
	return nil
}

func (gb *GoNotificationBlock) Deadlines() []Deadline {
	return []Deadline{}
}

func (gb *GoNotificationBlock) UpdateManual() bool {
	return false
}

func (gb *GoNotificationBlock) GetStatus() Status {
	return StatusFinished
}

func (gb *GoNotificationBlock) GetTaskHumanStatus() TaskHumanStatus {
	return ""
}

func (gb *GoNotificationBlock) compileText(ctx context.Context) (string, []email.Attachment, error) {
	author, err := gb.RunContext.People.GetUser(ctx, gb.RunContext.Initiator)
	if err != nil {
		return "", nil, err
	}

	typedAuthor, err := author.ToSSOUserTyped()
	if err != nil {
		return "", nil, err
	}

	text := mail.MakeBodyHeader(typedAuthor.Username, typedAuthor.Attributes.FullName,
		gb.RunContext.Sender.GetApplicationLink(gb.RunContext.WorkNumber), gb.State.Text)

	body, err := gb.RunContext.Storage.GetTaskRunContext(ctx, gb.RunContext.WorkNumber)
	if err != nil {
		return "", nil, err
	}

	descr := mail.MakeDescription(body.InitialApplication.ApplicationBody)
	text = mail.WrapDescription(text, descr)

	aa := mail.GetAttachmentsFromBody(body.InitialApplication.ApplicationBody, body.InitialApplication.AttachmentFields)

	attachments, err := gb.RunContext.FileRegistry.GetAttachments(ctx, aa)
	if err != nil {
		return "", nil, err
	}

	files := make([]email.Attachment, 0)
	for k := range attachments {
		files = append(files, attachments[k]...)
	}

	files, skippedFiles := sortAndFilterAttachments(files)

	text = mail.SwapKeys(text, body.InitialApplication.Keys)

	if len(skippedFiles) > 0 {
		text = fmt.Sprintf("%s <p>%s</p>", text, "Список файлов, которые не были доставлены в нотификацию:")
		for i := range skippedFiles {
			text = fmt.Sprintf("%s <p>%d. %s</p>", text, i+1, skippedFiles[i])
		}
	}

	text = mail.AddStyles(text)
	return text, files, nil
}

func (gb *GoNotificationBlock) Next(_ *store.VariableStore) ([]string, bool) {
	nexts, ok := script.GetNexts(gb.Sockets, DefaultSocketID)
	if !ok {
		return nil, false
	}
	return nexts, true
}

func (gb *GoNotificationBlock) GetState() interface{} {
	return gb.State
}

func (gb *GoNotificationBlock) Update(ctx context.Context) (interface{}, error) {
	emails := make([]string, 0, len(gb.State.People)+len(gb.State.Emails))

	for _, person := range gb.State.People {
		emailAddr := ""
		emailAddr, err := gb.RunContext.People.GetUserEmail(ctx, person)
		if err != nil {
			log.Println("can't get email of user", person)
			continue
		}
		emails = append(emails, emailAddr)
	}
	emails = append(emails, gb.State.Emails...)

	if len(emails) == 0 {
		return nil, errors.New("can't find any working emails from logins")
	}

	text, files, err := gb.compileText(ctx)
	if err != nil {
		return nil, errors.New("couldn't compile notification text")
	}

	err = gb.RunContext.Sender.SendNotification(ctx, emails, files, mail.Template{
		Subject:   gb.State.Subject,
		Text:      text,
		Variables: nil,
	})
	return nil, err
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
		Sockets: []script.Socket{script.DefaultSocket},
	}
}

// nolint:dupl // another block
func createGoNotificationBlock(name string, ef *entity.EriusFunc, runCtx *BlockRunContext) (*GoNotificationBlock, error) {
	b := &GoNotificationBlock{
		Name:    name,
		Title:   ef.Title,
		Input:   map[string]string{},
		Output:  map[string]string{},
		Sockets: entity.ConvertSocket(ef.Sockets),

		RunContext: runCtx,
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
	b.RunContext.VarStore.AddStep(b.Name)

	return b, nil
}

func sortAndFilterAttachments(files []email.Attachment) (res []email.Attachment, skippedFiles []string) {
	const attachmentsLimitMB = 35
	var limitCounter float64
	skippedFiles = make([]string, 0)

	sort.Slice(files, func(i, j int) bool {
		return len(files[i].Content) < len(files[j].Content)
	})

	res = make([]email.Attachment, 0, len(files))
	for i := range files {
		limitCounter += float64(len(files[i].Content) / 1024 / 1024)
		if limitCounter <= attachmentsLimitMB {
			res = append(res, files[i])
		} else {
			skippedFiles = append(skippedFiles, files[i].Name)
		}
	}

	return res, skippedFiles
}
