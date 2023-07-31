package pipeline

import (
	c "context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
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

	signActionSign   = "sign"
	signActionReject = "reject"
	signActionError  = "error"
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
			key = executedSocketID
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
			return StatusSignRejected
		}

		if *gb.State.Decision == SignDecisionError {
			return StatusProcessingError
		}

		return StatusSignSigned
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
	for i := 0; i < len(gb.State.SignLog); i++ {
		log := gb.State.SignLog[i]
		if log.Login == login {
			return true
		}
	}
	return false
}

func (gb *GoSignBlock) signActions() []MemberAction {
	if gb.State.Decision != nil {
		return nil
	}

	return []MemberAction{
		{
			Id:   signActionSign,
			Type: ActionTypePrimary,
		},
		{
			Id:   signActionReject,
			Type: ActionTypeSecondary,
		},
		{
			Id:   signActionError,
			Type: ActionTypeOther,
		}}
}

func (gb *GoSignBlock) Members() []Member {
	members := make([]Member, 0)
	for login := range gb.State.Signers {
		members = append(members, Member{
			Login:      login,
			IsFinished: gb.isSignerFinished(login),
			Actions:    gb.signActions(),
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

	return nil
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

// nolint:dupl // another block
func createGoSignBlock(ctx c.Context, name string, ef *entity.EriusFunc, runCtx *BlockRunContext) (*GoSignBlock, bool, error) {
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

	if err := b.createState(ctx, ef); err != nil {
		return nil, false, err
	}
	b.RunContext.VarStore.AddStep(b.Name)

	return b, false, nil
}
