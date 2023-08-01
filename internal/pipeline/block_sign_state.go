package pipeline

import (
	"errors"
	"fmt"
	"time"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

type SignDecision string

func (a SignDecision) String() string {
	return string(a)
}

type SignLogEntry struct {
	Login     string       `json:"login"`
	Decision  SignDecision `json:"decision"`
	Comment   string       `json:"comment"`
	CreatedAt time.Time    `json:"created_at"`
}

type SignData struct {
	Type             script.SignerType       `json:"type"`
	Signers          map[string]struct{}     `json:"signers"`
	Decision         *SignDecision           `json:"decision,omitempty"`
	Comment          *string                 `json:"comment,omitempty"`
	ActualSigner     *string                 `json:"actual_signer,omitempty"`
	SigningRule      script.SigningRule      `json:"signingRule,omitempty"`
	SignatureCarrier script.SignatureCarrier `json:"signature_carrier,omitempty"`
	SignLog          []SignLogEntry          `json:"sign_log,omitempty"`

	FormsAccessibility []script.FormAccessibility `json:"forms_accessibility,omitempty"`

	SignerGroupID   string `json:"signer_group_id,omitempty"`
	SignerGroupName string `json:"signer_group_name,omitempty"`
}

func (s *SignData) SetDecision(login string, params *SignSignatureParams) error {
	_, signerFound := s.Signers[login]
	if !signerFound {
		return NewUserIsNotPartOfProcessErr()
	}

	switch params.Decision {
	case "":
		return errors.New("missing decision")
	case SignDecisionSigned, SignDecisionRejected, SignDecisionError:
	default:
		return errors.New("unknown decision")
	}

	if s.Decision != nil {
		return errors.New("decision already set")
	}

	var signingRule = s.SigningRule

	if params.Decision == SignDecisionSigned {
		params.Comment = ""
	}

	if signingRule == script.AnyOfSigningRequired {
		s.Decision = &params.Decision
		s.Comment = &params.Comment
		s.ActualSigner = &login

		var signingLogEntry = SignLogEntry{
			Login:     login,
			Decision:  params.Decision,
			Comment:   params.Comment,
			CreatedAt: time.Now(),
		}

		s.SignLog = append(s.SignLog, signingLogEntry)
	}

	if signingRule == script.AllOfSigningRequired {
		for i := 0; i < len(s.SignLog); i++ {
			entry := s.SignLog[i]
			if entry.Login == login {
				return errors.New(fmt.Sprintf("decision of user %s is already set", login))
			}
		}

		var signingLogEntry = SignLogEntry{
			Login:     login,
			Decision:  params.Decision,
			Comment:   params.Comment,
			CreatedAt: time.Now(),
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

		s.Decision = &overallDecision
	}

	return nil
}
