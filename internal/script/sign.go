package script

import (
	"errors"
	"fmt"
	"strings"
)

type SigningRule string

func (s SigningRule) String() string {
	return string(s)
}

type SignerType string

func (s SignerType) String() string {
	return string(s)
}

type SignatureType string

func (s SignatureType) String() string {
	return string(s)
}

type SignatureCarrier string

func (s SignatureCarrier) String() string {
	return string(s)
}

const (
	SignerTypeUser       SignerType = "user"
	SignerTypeGroup      SignerType = "group"
	SignerTypeFromSchema SignerType = "fromSchema"

	AllOfSigningRequired SigningRule = "AllOf"
	AnyOfSigningRequired SigningRule = "AnyOf"

	SignatureTypePEP  SignatureType = "pep"
	SignatureTypeUNEP SignatureType = "unep"
	SignatureTypeUKEP SignatureType = "ukep"

	SignatureCarrierCloud SignatureCarrier = "cloud"
	SignatureCarrierToken SignatureCarrier = "token"
	SignatureCarrierAll   SignatureCarrier = "all"
)

type SignParams struct {
	Type             SignerType       `json:"signerType"`
	SigningRule      SigningRule      `json:"signingRule"`
	Signer           string           `json:"signer,omitempty"`
	SignatureType    SignatureType    `json:"signatureType"`
	SignatureCarrier SignatureCarrier `json:"signatureCarrier,omitempty"`

	SignerGroupID     string `json:"signerGroupId,omitempty"`
	SignerGroupName   string `json:"signerGroupName,omitempty"`
	SignerGroupIDPath string `json:"signerGroupIdPath,omitempty"`

	FormsAccessibility []FormAccessibility `json:"formsAccessibility"`

	SLA        *int    `json:"sla"`
	CheckSLA   *bool   `json:"check_sla"`
	AutoReject *bool   `json:"auto_reject"`
	WorkType   *string `json:"work_type"`
}

func (s *SignParams) checkSignerTypeUserValid() error {
	if s.Signer == "" {
		return errors.New("signer is empty")
	}
	return nil
}

func (s *SignParams) checkSignerTypeGroupValid() error {
	if s.SignerGroupID == "" && s.SignerGroupIDPath == "" {
		return errors.New("signer group id is empty")
	}
	if s.SigningRule != "" && s.SigningRule != AllOfSigningRequired && s.SigningRule != AnyOfSigningRequired {
		return fmt.Errorf("unknown signing rule: %s", s.SigningRule)
	}
	return nil
}

func (s *SignParams) checkSignerTypeFromSchemaValid() error {
	if s.Signer == "" {
		return errors.New("signer is empty")
	}
	if len(strings.Split(s.Signer, ";")) > 1 {
		if s.SigningRule != "" && s.SigningRule != AllOfSigningRequired && s.SigningRule != AnyOfSigningRequired {
			return fmt.Errorf("unknown signing rule: %s", s.SigningRule)
		}
	}
	return nil
}

func (s *SignParams) checkSignerTypeValid() error {
	switch s.Type {
	case SignerTypeUser:
		if err := s.checkSignerTypeUserValid(); err != nil {
			return err
		}
		return nil
	case SignerTypeGroup:
		if err := s.checkSignerTypeGroupValid(); err != nil {
			return err
		}
		return nil
	case SignerTypeFromSchema:
		if err := s.checkSignerTypeFromSchemaValid(); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("unknown signer type: %s", s.Type)
	}
}

func (s *SignParams) Validate() error {
	switch s.SignatureType {
	case SignatureTypePEP:
		if s.Type != SignerTypeUser {
			return fmt.Errorf("bad signer type: %s", s.Type)
		}
		if err := s.checkSignerTypeUserValid(); err != nil {
			return err
		}
	case SignatureTypeUNEP:
		if err := s.checkSignerTypeValid(); err != nil {
			return err
		}
	case SignatureTypeUKEP:
		if err := s.checkSignerTypeValid(); err != nil {
			return err
		}
		if s.SignatureCarrier == "" {
			return errors.New("no signature carrier provided")
		}
		carrier := s.SignatureCarrier
		if carrier != SignatureCarrierCloud && carrier != SignatureCarrierToken && carrier != SignatureCarrierAll {
			return fmt.Errorf("unknown signature carrier: %s", s.SignatureCarrier)
		}
	default:
		return fmt.Errorf("unknown signature type: %s", s.SignatureType)
	}

	if s.CheckSLA != nil && *s.CheckSLA {
		if s.SLA == nil || *s.SLA == 0 {
			return errors.New("sla can`t be zero or nil")
		}

		if s.WorkType == nil || *s.WorkType == "" {
			return errors.New("work type can`t be empty or nil")
		}

		if s.AutoReject == nil {
			return errors.New("auto reject can`t be nil")
		}
	}
	return nil
}
