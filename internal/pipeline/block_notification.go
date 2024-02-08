package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/iancoleman/orderedmap"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/abp/mail/pkg/email"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	file_registry "gitlab.services.mts.ru/jocasta/pipeliner/internal/fileregistry"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type NotificationData struct {
	People          []string            `json:"people"`
	Emails          []string            `json:"emails"`
	UsersFromSchema map[string]struct{} `json:"usersFromSchema"`
	Subject         string              `json:"subject"`
	Text            string              `json:"text"`
	TextSource      script.TextSource   `json:"textSource"`
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

func (gb *GoNotificationBlock) GetTaskHumanStatus() (status TaskHumanStatus, comment, action string) {
	return "", "", ""
}

func (gb *GoNotificationBlock) compileText(ctx context.Context) (*mail.Notif, []email.Attachment, error) {
	author, err := gb.RunContext.Services.People.GetUser(ctx, gb.RunContext.Initiator)
	if err != nil {
		return nil, nil, err
	}

	typedAuthor, err := author.ToUserinfo()
	if err != nil {
		return nil, nil, err
	}

	description, files, err := gb.RunContext.makeNotificationDescription(gb.Name)
	if err != nil {
		return nil, nil, err
	}

	text, err := gb.notificationBlockText()
	if err != nil {
		return nil, nil, err
	}

	tpl := &mail.Notif{
		Title:       gb.State.Subject,
		Body:        text,
		Description: description,
		Link:        gb.RunContext.Services.Sender.GetApplicationLink(gb.RunContext.WorkNumber),
		Initiator:   typedAuthor,
	}

	return tpl, files, nil
}

func (gb *GoNotificationBlock) notificationBlockText() (string, error) {
	switch gb.State.TextSource.Type() {
	case script.OwnValueSource:
		return gb.ownValueSourceText(), nil
	case script.ContextValueSource:
		return gb.contextValueSourceText()
	default:
		return "", script.ErrUnknownTextSourceType
	}
}

func (gb *GoNotificationBlock) ownValueSourceText() string {
	if gb.State.TextSource.Text != "" {
		return gb.State.TextSource.Text
	}

	return gb.State.Text
}

func (gb *GoNotificationBlock) contextValueSourceText() (string, error) {
	value, err := gb.RunContext.VarStore.GetString(gb.State.TextSource.RefValue)
	if err != nil {
		return "", err
	}

	// на фронте значение текст всегда оборачивается тегом p
	// <p>Hello Text!</p>
	return wrapWithTag("p", value), nil
}

func wrapWithTag(tag, text string) string {
	tagStart, tagEnd := makeTag(tag)
	if !strings.HasPrefix(text, tagStart) {
		text = tagStart + text
	}

	if !strings.HasSuffix(text, tagEnd) {
		text += tagEnd
	}

	return text
}

func makeTag(tag string) (tagStart, tagEnd string) {
	return fmt.Sprintf("<%s>", tag), fmt.Sprintf("</%s>", tag)
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
		emailAddr, err := gb.RunContext.Services.People.GetUserEmail(ctx, person)
		if err != nil {
			log.Println("can't get email of user", person)

			continue
		}

		emails = append(emails, emailAddr)
	}

	emails = append(emails, gb.State.Emails...)

	for person := range gb.State.UsersFromSchema {
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
		return nil, errors.New("couldn't compile notification text")
	}

	iconsName := make([]string, 0, 1)

	for _, v := range text.Description {
		links, link := v.Get("attachLinks")
		if link {
			attachFiles, ok := links.([]file_registry.AttachInfo)
			if ok && len(attachFiles) != 0 {
				descIcons := []string{downloadImg}
				iconsName = append(iconsName, descIcons...)

				break
			}
		}
	}

	text.Description = mail.CheckGroup(text.Description)
	tpl := mail.Template{
		Subject:  gb.State.Subject,
		Template: "internal/mail/template/28email-template.html",
		Image:    "28_e-mail.png",
		Variables: struct {
			Title       string
			Body        string
			Initiator   *sso.UserInfo
			Description []orderedmap.OrderedMap
			Link        string
		}{
			Title:       text.Title,
			Body:        text.Body,
			Initiator:   text.Initiator,
			Description: text.Description,
			Link:        text.Link,
		},
	}

	fileNames := []string{tpl.Image, userImg}
	fileNames = append(fileNames, iconsName...)

	filesAttach, err := gb.RunContext.GetIcons(fileNames)
	if err != nil {
		return nil, errors.New("couldn't get icons")
	}

	files = append(files, filesAttach...)

	err = gb.RunContext.Services.Sender.SendNotification(ctx, emails, files, tpl)
	if err != nil {
		log.Println(err)

		return nil, err
	}

	if _, oks := gb.expectedEvents[eventEnd]; oks {
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
				TextSource: script.TextSource{
					SourceType: "",
					Text:       "",
					RefValue:   "",
				},
			},
		},
		Sockets: []script.Socket{script.DefaultSocket},
	}
}

// nolint:dupl,unparam // another block
func createGoNotificationBlock(
	ctx context.Context,
	name string,
	ef *entity.EriusFunc,
	runCtx *BlockRunContext,
	expectedEvents map[string]struct{},
) (*GoNotificationBlock, bool, error) {
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
		//nolint:gocritic //в этом проекте не принято использовать поинтеры в коллекциях
		for propertyName, v := range ef.Output.Properties {
			b.Output[propertyName] = v.Global
		}
	}

	var params script.NotificationParams

	err := json.Unmarshal(ef.Params, &params)
	if err != nil {
		return nil, reEntry, errors.Wrap(err, "can not get notification parameters")
	}

	if err = params.Validate(); err != nil {
		return nil, reEntry, errors.Wrap(err, "invalid notification parameters")
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
		TextSource:      params.TextSource,
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
			requiredFiles = append(requiredFiles, entity.Attachment{FileID: files[i].FileID})
		} else {
			skippedFiles = append(skippedFiles, file_registry.AttachInfo{FileID: files[i].FileID, Name: files[i].Name, Size: files[i].Size})
		}
	}

	return requiredFiles, skippedFiles
}
