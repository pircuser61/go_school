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

const (
	SignerTypeUser       SignerType = "user"
	SignerTypeGroup      SignerType = "group"
	SignerTypeFromSchema SignerType = "fromSchema"

	AllOfSigningRequired SigningRule = "AllOf"
	AnyOfSigningRequired SigningRule = "AnyOf"

	SignatureTypePEP  = "pep"
	SignatureTypeUNEP = "unep"
	SignatureTypeUKEP = "ukep"
)

type SignParams struct {
	Type          SignerType    `json:"signerType"`
	SigningRule   SigningRule   `json:"signingRule"`
	Signer        string        `json:"signer,omitempty"`
	SignatureType SignatureType `json:"signatureType"`

	SignerGroupID     string `json:"signerGroupId,omitempty"`
	SignerGroupName   string `json:"signerGroupName,omitempty"`
	SignerGroupIDPath string `json:"signerGroupIdPath,omitempty"`

	FormsAccessibility []FormAccessibility `json:"formsAccessibility"`
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
	case SignatureTypeUNEP, SignatureTypeUKEP:
		if err := s.checkSignerTypeValid(); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown signature type: %s", s.SignatureType)
	}

	return nil
}
