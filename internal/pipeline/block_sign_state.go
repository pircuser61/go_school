package pipeline

import (
	"errors"
	"fmt"
	"time"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

type SignerLogType string

const (
	SignerLogDecision           SignerLogType = "decision"
	AdditionalSignerLogDecision SignerLogType = "additionalApproverDecision"
	SignerLogAddApprover        SignerLogType = "addApprover"

	SignDecisionAddApproverApprovedRU = "согласен"
	SignDecisionAddApproverRejectedRU = "не согласен"
)

type SignDecision string

func (sd SignDecision) String() string {
	return string(sd)
}

func (sd SignDecision) ToRuString() string {
	//nolint:exhaustive //не хотим обрабатывать остальные случаи
	switch sd {
	case SignDecisionRejected:
		return SignDecisionAddApproverRejectedRU
	case SignDecisionAddApproverApproved:
		return SignDecisionAddApproverApprovedRU
	default:
		return string(sd)
	}
}

type SignLogEntry struct {
	Login          string              `json:"login"`
	Decision       SignDecision        `json:"decision"`
	Comment        string              `json:"comment"`
	CreatedAt      time.Time           `json:"created_at"`
	Attachments    []entity.Attachment `json:"attachments,omitempty"`
	AddedApprovers []string            `json:"added_approvers"`
	LogType        SignerLogType       `json:"log_type"`
}

type SigningParams struct {
	INN   string              `json:"inn"`
	SNILS string              `json:"snils"`
	Files []entity.Attachment `json:"files"`
}

type SignData struct {
	Type               script.SignerType         `json:"type"`
	Signers            map[string]struct{}       `json:"signers"`
	SignatureType      script.SignatureType      `json:"signature_type"`
	Decision           *SignDecision             `json:"decision,omitempty"`
	Comment            *string                   `json:"comment,omitempty"`
	ActualSigner       *string                   `json:"actual_signer,omitempty"`
	Attachments        []entity.Attachment       `json:"attachments,omitempty"`
	Signatures         []fileSignaturePair       `json:"signatures,omitempty"`
	SigningParams      SigningParams             `json:"signing_params,omitempty"`
	SigningParamsPaths script.SigningParamsPaths `json:"signing_params_paths,omitempty"`
	SigningRule        script.SigningRule        `json:"signing_rule,omitempty"`
	SignatureCarrier   script.SignatureCarrier   `json:"signature_carrier,omitempty"`
	SignLog            []SignLogEntry            `json:"sign_log,omitempty"`

	FormsAccessibility []script.FormAccessibility `json:"forms_accessibility,omitempty"`

	IsExpired     bool   `json:"is_expired"`
	IsTakenInWork bool   `json:"is_taken_in_work"`
	WorkerLogin   string `json:"worker_login"`

	SignerGroupID   string `json:"signer_group_id,omitempty"`
	SignerGroupName string `json:"signer_group_name,omitempty"`

	Deadline   time.Time `json:"deadline,omitempty"`
	SLA        *int      `json:"sla,omitempty"`
	CheckSLA   *bool     `json:"check_sla,omitempty"`
	AutoReject *bool     `json:"auto_reject,omitempty"`
	WorkType   *string   `json:"work_type,omitempty"`

	SLAChecked          bool `json:"sla_checked"`
	DayBeforeSLAChecked bool `json:"before_day_sla_checked"`

	AdditionalApprovers []AdditionalSignApprover `json:"additional_approvers,omitempty"`

	Reentered bool `json:"reentered"`
}

type AdditionalSignApprover struct {
	ApproverLogin string              `json:"approver_login"`
	BaseLogin     string              `json:"base_login"`
	Question      *string             `json:"question"`
	Comment       *string             `json:"comment"`
	Attachments   []entity.Attachment `json:"attachments"`
	Decision      *SignDecision       `json:"decision"`
	CreatedAt     time.Time           `json:"created_at"`
	DecisionTime  *time.Time          `json:"decision_time"`
}

func (s *SignData) userIsSignerOrAddApprover(login string) bool {
	if login == autoSigner {
		return true
	}

	_, ok := s.Signers[login]
	if ok {
		return true
	}

	for _, signer := range s.AdditionalApprovers {
		if signer.Decision == nil && signer.ApproverLogin == login {
			return true
		}
	}

	return false
}

func (s *SignData) handleAnyOfDecision(login string, params *signSignatureParams) {
	s.Decision = &params.Decision
	s.Comment = &params.Comment
	s.ActualSigner = &login

	signingLogEntry := SignLogEntry{
		Login:       login,
		Decision:    params.Decision,
		Comment:     params.Comment,
		CreatedAt:   time.Now(),
		Attachments: params.Attachments,
		LogType:     SignerLogDecision,
	}

	s.SignLog = append(s.SignLog, signingLogEntry)
}

func (s *SignData) handleAllOfDecision(login string, params *signSignatureParams) error {
	for i := 0; i < len(s.SignLog); i++ {
		entry := s.SignLog[i]
		if entry.LogType != SignerLogDecision {
			continue
		}

		if entry.Login == login {
			return fmt.Errorf("decision of user %s is already set", login)
		}
	}

	signingLogEntry := SignLogEntry{
		Login:       login,
		Decision:    params.Decision,
		Comment:     params.Comment,
		CreatedAt:   time.Now(),
		Attachments: params.Attachments,
		LogType:     SignerLogDecision,
	}

	s.SignLog = append(s.SignLog, signingLogEntry)

	var overallDecision SignDecision

	//nolint:exhaustive // не хотим обратывать остальные случаи
	switch params.Decision {
	case SignDecisionRejected:
		overallDecision = SignDecisionRejected
	case SignDecisionError:
		overallDecision = SignDecisionError
	default:
		var decisionCount int

		//nolint:gocritic //в этом проекте не принято использовать поинтеры в коллекциях
		for _, log := range s.SignLog {
			if log.LogType == SignerLogDecision {
				decisionCount++
			}
		}

		if decisionCount == len(s.Signers) {
			overallDecision = SignDecisionSigned
		}
	}

	if overallDecision != "" {
		s.Decision = &overallDecision
		s.Comment = &params.Comment
		s.ActualSigner = &login
	}

	return nil
}

func (s *SignData) SetDecision(login string, params *signSignatureParams) error {
	isAutoDecision := login == autoSigner

	if isAutoDecision {
		s.handleAnyOfDecision(login, params)

		return nil
	}

	//nolint:exhaustive // не надо обрабатывать эти случаи значит не надо
	switch params.Decision {
	case "":
		return errors.New("missing decision")
	case SignDecisionSigned, SignDecisionRejected, SignDecisionError:
	default:
		return errors.New("unknown decision")
	}

	if s.SignatureType == script.SignatureTypeUKEP && params.Decision == SignDecisionSigned &&
		s.SignatureCarrier == script.SignatureCarrierToken && len(params.Signatures) == 0 {
		return errors.New("attachments for ukep token signing are required")
	}

	if s.Decision != nil {
		return errors.New("decision already set")
	}

	signingRule := s.SigningRule

	if params.Decision == SignDecisionSigned {
		params.Comment = ""
	}

	if signingRule == script.AnyOfSigningRequired {
		s.handleAnyOfDecision(login, params)
	}

	if signingRule == script.AllOfSigningRequired {
		if err := s.handleAllOfDecision(login, params); err != nil {
			return err
		}
	}

	return nil
}

func (s *SignData) SetDecisionByAdditionalApprover(
	login string,
	params additionalApproverSignUpdateParams,
) ([]string, error) {
	approverFound := s.checkForAdditionalApprover(login)
	if !approverFound {
		return nil, NewUserIsNotPartOfProcessErr()
	}

	if s.Decision != nil {
		return nil, errors.New("decision already set")
	}

	loginsToNotify := make([]string, 0)
	couldUpdateOne := false
	timeNow := time.Now()

	for i := range s.AdditionalApprovers {
		if login != s.AdditionalApprovers[i].ApproverLogin || s.AdditionalApprovers[i].Decision != nil {
			continue
		}

		s.AdditionalApprovers[i].Decision = &params.Decision
		s.AdditionalApprovers[i].Comment = &params.Comment
		s.AdditionalApprovers[i].Attachments = params.Attachments

		if s.AdditionalApprovers[i].DecisionTime == nil {
			s.AdditionalApprovers[i].DecisionTime = &timeNow
		}

		signerLogEntry := SignLogEntry{
			Login:       login,
			Decision:    params.Decision,
			Comment:     params.Comment,
			Attachments: params.Attachments,
			CreatedAt:   time.Now(),
			LogType:     AdditionalSignerLogDecision,
		}

		s.SignLog = append(s.SignLog, signerLogEntry)
		loginsToNotify = append(loginsToNotify, s.AdditionalApprovers[i].BaseLogin)
		couldUpdateOne = true
	}

	if !couldUpdateOne {
		return nil, fmt.Errorf("can't approve any request")
	}

	return loginsToNotify, nil
}

func (s *SignData) checkForAdditionalApprover(login string) bool {
	for _, signer := range s.AdditionalApprovers {
		if login == signer.ApproverLogin {
			return true
		}
	}

	return false
}
