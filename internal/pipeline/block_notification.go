package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/abp/mail/pkg/email"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	file_registry "gitlab.services.mts.ru/jocasta/pipeliner/internal/file-registry"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type NotificationData struct {
	People          []string            `json:"people"`
	Emails          []string            `json:"emails"`
	UsersFromSchema map[string]struct{} `json:"usersFromSchema"`
	Subject         string              `json:"subject"`
	Text            string              `json:"text"`
}

type GoNotificationBlock struct {
	Name      string
	ShortName string
	Title     string
	Input     map[string]string
	Output    map[string]string
	Sockets   []script.Socket
	State     *NotificationData

	RunContext *BlockRunContext

	expectedEvents map[string]struct{}
	happenedEvents []entity.NodeEvent
}

func (gb *GoNotificationBlock) GetNewEvents() []entity.NodeEvent {
	return gb.happenedEvents
}

func (gb *GoNotificationBlock) Members() []Member {
	return nil
}

func (gb *GoNotificationBlock) Deadlines(_ context.Context) ([]Deadline, error) {
	return []Deadline{}, nil
}

func (gb *GoNotificationBlock) UpdateManual() bool {
	return false
}

func (gb *GoNotificationBlock) GetStatus() Status {
	return StatusFinished
}

func (gb *GoNotificationBlock) GetTaskHumanStatus() (status TaskHumanStatus, comment string, action string) {
	return "", "", ""
}

func (gb *GoNotificationBlock) compileText(ctx context.Context) (string, []email.Attachment, error) {
	author, err := gb.RunContext.Services.People.GetUser(ctx, gb.RunContext.Initiator)
	if err != nil {
		return "", nil, err
	}

	typedAuthor, err := author.ToSSOUserTyped()
	if err != nil {
		return "", nil, err
	}

	text := mail.MakeBodyHeader(typedAuthor.Username, typedAuthor.Attributes.FullName,
		gb.RunContext.Services.Sender.GetApplicationLink(gb.RunContext.WorkNumber), gb.State.Text)

	body, err := gb.RunContext.Services.Storage.GetTaskRunContext(ctx, gb.RunContext.WorkNumber)
	if err != nil {
		return "", nil, err
	}

	descr := mail.MakeDescription(body.InitialApplication.ApplicationBody)
	text = mail.WrapDescription(text, descr)

	aa := mail.GetAttachmentsFromBody(body.InitialApplication.ApplicationBody)

	attachmentsInfo, err := gb.RunContext.Services.FileRegistry.GetAttachmentsInfo(ctx, aa)
	if err != nil {
		return "", nil, err
	}

	filesInfo := make([]file_registry.FileInfo, 0)
	for k := range attachmentsInfo {
		filesInfo = append(filesInfo, attachmentsInfo[k]...)
	}

	requiredFiles, skippedFiles := sortAndFilterAttachments(filesInfo)

	files, err := gb.RunContext.Services.FileRegistry.GetAttachments(ctx, requiredFiles)
	if err != nil {
		return "", nil, err
	}

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
		emailAddr, err := gb.RunContext.Services.People.GetUserEmail(ctx, person)
		if err != nil {
			log.Println("can't get email of user", person)
			continue
		}
		emails = append(emails, emailAddr)
	}
	emails = append(emails, gb.State.Emails...)

	for person, _ := range gb.State.UsersFromSchema {
		emailAddr := ""
		emailAddr, err := gb.RunContext.Services.People.GetUserEmail(ctx, person)
		if err != nil {
			log.Println("can't get email of user", person)
			continue
		}
		emails = append(emails, emailAddr)
	}

	if len(emails) == 0 {
		return nil, errors.New("can't find any working emails from logins")
	}

	text, files, err := gb.compileText(ctx)
	if err != nil {
		return nil, errors.New("couldn't compile template text")
	}

	err = gb.RunContext.Services.Sender.SendNotification(ctx, emails, files, mail.Template{
		Subject:   gb.State.Subject,
		Template:  text,
		Variables: nil,
	})

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
				People:          []string{},
				Emails:          []string{},
				UsersFromSchema: "",
				Subject:         "",
				Text:            "",
			},
		},
		Sockets: []script.Socket{script.DefaultSocket},
	}
}

// nolint:dupl,unparam // another block
func createGoNotificationBlock(ctx context.Context, name string, ef *entity.EriusFunc, runCtx *BlockRunContext,
	expectedEvents map[string]struct{}) (*GoNotificationBlock, bool, error) {
	const reEntry = false

	b := &GoNotificationBlock{
		Name:      name,
		ShortName: ef.ShortTitle,
		Title:     ef.Title,
		Input:     map[string]string{},
		Output:    map[string]string{},
		Sockets:   entity.ConvertSocket(ef.Sockets),

		RunContext: runCtx,

		expectedEvents: expectedEvents,
		happenedEvents: make([]entity.NodeEvent, 0),
	}

	for _, v := range ef.Input {
		b.Input[v.Name] = v.Global
	}

	if ef.Output != nil {
		for propertyName, v := range ef.Output.Properties {
			b.Output[propertyName] = v.Global
		}
	}

	var params script.NotificationParams
	err := json.Unmarshal(ef.Params, &params)
	if err != nil {
		return nil, reEntry, errors.Wrap(err, "can not get template parameters")
	}

	if err = params.Validate(); err != nil {
		return nil, reEntry, errors.Wrap(err, "invalid template parameters")
	}

	variableStorage, grabStorageErr := b.RunContext.VarStore.GrabStorage()
	if grabStorageErr != nil {
		return nil, reEntry, errors.Wrap(err, "can not create GrabStorage")
	}

	usersFromSchema := make(map[string]struct{})

	if params.UsersFromSchema != "" {
		usersVars := strings.Split(params.UsersFromSchema, ";")
		for i := range usersVars {
			resolvedEntities, resolveErr := getUsersFromVars(
				variableStorage,
				map[string]struct{}{
					usersVars[i]: {},
				},
			)
			if resolveErr != nil {
				return nil, reEntry, errors.Wrap(resolveErr, "can not get users from vars")
			}

			for userLogin := range resolvedEntities {
				usersFromSchema[userLogin] = struct{}{}
			}
		}
	}

	b.State = &NotificationData{
		People:          params.People,
		Emails:          params.Emails,
		Text:            params.Text,
		Subject:         params.Subject,
		UsersFromSchema: usersFromSchema,
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

func sortAndFilterAttachments(files []file_registry.FileInfo) (requiredFiles []entity.Attachment, skippedFiles []file_registry.AttachInfo) {
	const attachmentsLimitMB = 20
	var limitCounter float64
	skippedFiles = make([]file_registry.AttachInfo, 0)

	sort.Slice(files, func(i, j int) bool {
		return files[i].Size < files[j].Size
	})

	requiredFiles = make([]entity.Attachment, 0, len(files))
	for i := range files {
		limitCounter += float64(files[i].Size) / 1024 / 1024
		if limitCounter <= attachmentsLimitMB {
			requiredFiles = append(requiredFiles, entity.Attachment{FileID: files[i].FileId})
		} else {
			skippedFiles = append(skippedFiles, file_registry.AttachInfo{FileID: files[i].FileId, Name: files[i].Name, Size: files[i].Size})
		}
	}

	return requiredFiles, skippedFiles
}
