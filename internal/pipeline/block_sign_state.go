package pipeline

import (
	"errors"
	"fmt"
	"time"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

type SignDecision string

func (a SignDecision) String() string {
	return string(a)
}

type SignLogEntry struct {
	Login       string              `json:"login"`
	Decision    SignDecision        `json:"decision"`
	Comment     string              `json:"comment"`
	CreatedAt   time.Time           `json:"created_at"`
	Attachments []entity.Attachment `json:"attachments,omitempty"`
}

type SignData struct {
	Type             script.SignerType       `json:"type"`
	Signers          map[string]struct{}     `json:"signers"`
	SignatureType    script.SignatureType    `json:"signature_type"`
	Decision         *SignDecision           `json:"decision,omitempty"`
	Comment          *string                 `json:"comment,omitempty"`
	ActualSigner     *string                 `json:"actual_signer,omitempty"`
	Attachments      []entity.Attachment     `json:"attachments,omitempty"`
	SigningRule      script.SigningRule      `json:"signing_rule,omitempty"`
	SignatureCarrier script.SignatureCarrier `json:"signature_carrier,omitempty"`
	SignLog          []SignLogEntry          `json:"sign_log,omitempty"`

	FormsAccessibility []script.FormAccessibility `json:"forms_accessibility,omitempty"`

	IsTakenInWork bool   `json:"is_taken_in_work"`
	WorkerLogin   string `json:"worker_login"`

	SignerGroupID   string `json:"signer_group_id,omitempty"`
	SignerGroupName string `json:"signer_group_name,omitempty"`

	SLA        *int    `json:"sla,omitempty"`
	CheckSLA   *bool   `json:"check_sla,omitempty"`
	AutoReject *bool   `json:"auto_reject,omitempty"`
	WorkType   *string `json:"work_type,omitempty"`

	SLAChecked          bool `json:"sla_checked"`
	DayBeforeSLAChecked bool `json:"before_day_sla_checked"`

	Reentered bool `json:"reentered"`
}

func (s *SignData) handleAnyOfDecision(login string, params *signSignatureParams) {
	s.Decision = &params.Decision
	s.Comment = &params.Comment
	s.ActualSigner = &login

	var signingLogEntry = SignLogEntry{
		Login:       login,
		Decision:    params.Decision,
		Comment:     params.Comment,
		CreatedAt:   time.Now(),
		Attachments: params.Attachments,
	}

	s.SignLog = append(s.SignLog, signingLogEntry)
}

func (s *SignData) handleAllOfDecision(login string, params *signSignatureParams) error {
	for i := 0; i < len(s.SignLog); i++ {
		entry := s.SignLog[i]
		if entry.Login == login {
			return errors.New(fmt.Sprintf("decision of user %s is already set", login))
		}
	}

	var signingLogEntry = SignLogEntry{
		Login:       login,
		Decision:    params.Decision,
		Comment:     params.Comment,
		CreatedAt:   time.Now(),
		Attachments: params.Attachments,
	}

	s.SignLog = append(s.SignLog, signingLogEntry)

	var overallDecision SignDecision

	switch params.Decision {
	case SignDecisionRejected:
		overallDecision = SignDecisionRejected
	case SignDecisionError:
		overallDecision = SignDecisionError
	default:
		if len(s.SignLog) == len(s.Signers) {
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

	switch params.Decision {
	case "":
		return errors.New("missing decision")
	case SignDecisionSigned, SignDecisionRejected, SignDecisionError:
	default:
		return errors.New("unknown decision")
	}

	if s.SignatureType == script.SignatureTypeUKEP && params.Decision == SignDecisionSigned &&
		s.SignatureCarrier == script.SignatureCarrierToken && len(params.Attachments) == 0 {
		return errors.New("attachments for ukep token signing are required")
	}

	if s.Decision != nil {
		return errors.New("decision already set")
	}

	var signingRule = s.SigningRule

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
