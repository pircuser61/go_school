package pipeline

import (
	c "context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"

	e "gitlab.services.mts.ru/abp/mail/pkg/email"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

const (
	keyOutputSigner       = "signer"
	keyOutputSignDecision = "decision"
	keyOutputSignComment  = "comment"

	SignDecisionSigned   SignDecision = "signed"
	SignDecisionRejected SignDecision = "rejected"
	SignDecisionError    SignDecision = "error"

	signActionSign   = "sign_sign"
	signActionReject = "sign_reject"
)

type GoSignBlock struct {
	Name    string
	Title   string
	Input   map[string]string
	Output  map[string]string
	Sockets []script.Socket
	State   *SignData

	RunContext *BlockRunContext
}

func (gb *GoSignBlock) GetState() interface{} {
	return gb.State
}

func (gb *GoSignBlock) Next(_ *store.VariableStore) ([]string, bool) {
	var key string
	if gb.State != nil && gb.State.Decision != nil {
		switch *gb.State.Decision {
		case SignDecisionSigned:
			key = signedSocketID
		case SignDecisionRejected:
			key = rejectedSocketID
		case SignDecisionError:
			key = errorSocketID
		}
	}

	nexts, ok := script.GetNexts(gb.Sockets, key)
	if !ok {
		return nil, false
	}
	return nexts, true
}

func (gb *GoSignBlock) GetTaskHumanStatus() TaskHumanStatus {
	if gb.State != nil && gb.State.Decision != nil {
		if *gb.State.Decision == SignDecisionRejected {
			return StatusRejected
		}

		if *gb.State.Decision == SignDecisionError {
			return StatusProcessingError
		}

		return StatusSigned
	}
	return StatusSigning
}

func (gb *GoSignBlock) GetStatus() Status {
	if gb.State != nil && gb.State.Decision != nil {
		if *gb.State.Decision == SignDecisionRejected {
			return StatusNoSuccess
		}

		if *gb.State.Decision == SignDecisionError {
			return StatusError
		}

		return StatusFinished
	}
	return StatusRunning
}

func (gb *GoSignBlock) UpdateManual() bool {
	return true
}

func (gb *GoSignBlock) isSignerFinished(login string) bool {
	if gb.State.Decision != nil {
		return true
	}

	for _, s := range gb.State.SignLog {
		if s.Login == login {
			return true
		}
	}

	return false
}

func (gb *GoSignBlock) signActions(login string) []MemberAction {
	if gb.State.Decision != nil {
		return []MemberAction{}
	}

	for _, s := range gb.State.SignLog {
		if s.Login == login {
			return []MemberAction{}
		}
	}

	return []MemberAction{
		{
			Id:   signActionSign,
			Type: ActionTypePrimary,
		},
		{
			Id:   signActionReject,
			Type: ActionTypeSecondary,
		}}
}

func (gb *GoSignBlock) Members() []Member {
	members := make([]Member, 0)
	for login := range gb.State.Signers {
		members = append(members, Member{
			Login:      login,
			IsFinished: gb.isSignerFinished(login),
			Actions:    gb.signActions(login),
		})
	}
	return members
}

func (gb *GoSignBlock) Deadlines(_ c.Context) ([]Deadline, error) {
	return nil, nil
}

type setSignersByParamsDTO struct {
	Type    script.SignerType
	GroupID string
	Signer  string
}

func (gb *GoSignBlock) setSignersByParams(ctx c.Context, dto *setSignersByParamsDTO) error {
	switch dto.Type {
	case script.SignerTypeUser:
		gb.State.Signers = map[string]struct{}{
			dto.Signer: {},
		}
	case script.SignerTypeGroup:
		workGroup, errGroup := gb.RunContext.ServiceDesc.GetWorkGroup(ctx, dto.GroupID)
		if errGroup != nil {
			return errors.Wrap(errGroup, "can`t get signer group with id: "+dto.GroupID)
		}

		if len(workGroup.People) == 0 {
			return errors.New("zero signers in group: " + dto.GroupID)
		}

		gb.State.Signers = make(map[string]struct{})
		for i := range workGroup.People {
			gb.State.Signers[workGroup.People[i].Login] = struct{}{}
		}
		gb.State.SignerGroupID = dto.GroupID
		gb.State.SignerGroupName = workGroup.GroupName
	case script.SignerTypeFromSchema:
		variableStorage, grabStorageErr := gb.RunContext.VarStore.GrabStorage()
		if grabStorageErr != nil {
			return grabStorageErr
		}

		approversFromSchema := make(map[string]struct{})

		approversVars := strings.Split(dto.Signer, ";")
		for i := range approversVars {
			resolvedEntities, resolveErr := getUsersFromVars(
				variableStorage,
				map[string]struct{}{
					approversVars[i]: {},
				},
			)
			if resolveErr != nil {
				return resolveErr
			}

			for approverLogin := range resolvedEntities {
				approversFromSchema[approverLogin] = struct{}{}
			}
		}

		gb.State.Signers = approversFromSchema
	}

	return nil
}

//nolint:dupl,gocyclo // maybe later
func (gb *GoSignBlock) handleNotifications(ctx c.Context) error {
	l := logger.GetLogger(ctx)

	if gb.RunContext.skipNotifications {
		return nil
	}

	signers := getSliceFromMapOfStrings(gb.State.Signers)
	var emailAttachment []e.Attachment

	description, err := gb.RunContext.makeNotificationDescription(gb.Name)
	if err != nil {
		return err
	}

	var emails = make(map[string]mail.Template, 0)
	for _, login := range signers {
		em, getUserEmailErr := gb.RunContext.People.GetUserEmail(ctx, login)
		if getUserEmailErr != nil {
			l.WithField("login", login).WithError(getUserEmailErr).Warning("couldn't get email")
			continue
		}

		emails[em] = mail.NewSignerNotificationTpl(
			gb.RunContext.WorkNumber,
			gb.RunContext.NotifName,
			gb.RunContext.Sender.SdAddress,
			description)

	}

	if len(emails) == 0 {
		return nil
	}

	for i := range emails {
		if sendErr := gb.RunContext.Sender.SendNotification(ctx, []string{i}, emailAttachment, emails[i]); sendErr != nil {
			return sendErr
		}
	}
	return nil
}

//nolint:dupl,gocyclo //its not duplicate
func (gb *GoSignBlock) createState(ctx c.Context, ef *entity.EriusFunc) error {
	var params script.SignParams
	err := json.Unmarshal(ef.Params, &params)
	if err != nil {
		return errors.Wrap(err, "can not get sign parameters")
	}

	if err = params.Validate(); err != nil {
		return errors.Wrap(err, "invalid sign parameters")
	}

	gb.State = &SignData{
		Type:               params.Type,
		SigningRule:        params.SigningRule,
		SignLog:            make([]SignLogEntry, 0),
		FormsAccessibility: params.FormsAccessibility,
	}

	if gb.State.SigningRule == "" {
		gb.State.SigningRule = script.AnyOfSigningRequired
	}

	if params.Type == script.SignerTypeGroup && params.SignerGroupIDPath != "" {
		variableStorage, grabStorageErr := gb.RunContext.VarStore.GrabStorage()
		if grabStorageErr != nil {
			return grabStorageErr
		}

		groupId := getVariable(variableStorage, params.SignerGroupIDPath)
		if groupId == nil {
			return errors.New("can't find group id in variables")
		}
		params.SignerGroupIDPath = fmt.Sprintf("%v", groupId)
	}

	setErr := gb.setSignersByParams(ctx, &setSignersByParamsDTO{
		Type:    params.Type,
		GroupID: params.SignerGroupID,
		Signer:  params.Signer,
	})
	if setErr != nil {
		return setErr
	}

	return gb.handleNotifications(ctx)
}

func (gb *GoSignBlock) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockGoSignID,
		BlockType: script.TypeGo,
		Title:     gb.Title,
		Inputs:    nil,
		Outputs: []script.FunctionValueModel{
			{
				Name:    keyOutputSigner,
				Type:    "string",
				Comment: "signer login",
			},
			{
				Name:    keyOutputSignDecision,
				Type:    "string",
				Comment: "sign result",
			},
			{
				Name:    keyOutputSignComment,
				Type:    "string",
				Comment: "sign comment",
			},
		},
		Params: &script.FunctionParams{
			Type: BlockGoSignID,
			Params: &script.SignParams{
				Type:               "",
				SignatureType:      "",
				FormsAccessibility: []script.FormAccessibility{},
			},
		},
		Sockets: []script.Socket{
			script.SignedSocket,
			script.RejectedSocket,
			script.ErrorSocket,
		},
	}
}

func (gb *GoSignBlock) loadState(raw json.RawMessage) error {
	return json.Unmarshal(raw, &gb.State)
}

// nolint:dupl,unparam // another block
func createGoSignBlock(ctx c.Context, name string, ef *entity.EriusFunc, runCtx *BlockRunContext) (*GoSignBlock, bool, error) {
	const reEntry = false
	b := &GoSignBlock{
		Name:       name,
		Title:      ef.Title,
		Input:      map[string]string{},
		Output:     map[string]string{},
		Sockets:    entity.ConvertSocket(ef.Sockets),
		RunContext: runCtx,
	}

	for _, v := range ef.Input {
		b.Input[v.Name] = v.Global
	}

	for _, v := range ef.Output {
		b.Output[v.Name] = v.Global
	}

	rawState, ok := runCtx.VarStore.State[name]
	if ok {
		if err := b.loadState(rawState); err != nil {
			return nil, reEntry, err
		}
	} else {
		if err := b.createState(ctx, ef); err != nil {
			return nil, reEntry, err
		}
		b.RunContext.VarStore.AddStep(b.Name)
	}

	return b, reEntry, nil
}
