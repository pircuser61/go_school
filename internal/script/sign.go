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
	SignerTypeFromSchema SignerType = "from_schema"

	AllOfSigningRequired SigningRule = "AllOf"
	AnyOfSigningRequired SigningRule = "AnyOf"

	SignatureTypePEP  = "pep"
	SignatureTypeUNEP = "unep"
	SignatureTypeUKEP = "ukep"

	SignatureCarrierCloud = "cloud"
	SignatureCarrierToken = "token"
	SignatureCarrierAll   = "all"
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
}

//nolint:gocyclo //it's ok
func (s *SignParams) Validate() error {
	switch s.SignatureType {
	case SignatureTypePEP:
		if s.Type != SignerTypeUser {
			return fmt.Errorf("bad signer type: %s", s.Type)
		}
		if s.Signer == "" {
			return errors.New("signer is empty")
		}
	case SignatureTypeUNEP:
		switch s.Type {
		case SignerTypeUser:
			if s.Signer == "" {
				return errors.New("signer is empty")
			}
		case SignerTypeGroup:
			if s.SignerGroupID == "" && s.SignerGroupIDPath == "" {
				return errors.New("signer group id is empty")
			}
			if s.SigningRule != "" && s.SigningRule != AllOfSigningRequired && s.SigningRule != AnyOfSigningRequired {
				return fmt.Errorf("unknown signing rule: %s", s.SigningRule)
			}
		case SignerTypeFromSchema:
			if s.Signer == "" {
				return errors.New("signer is empty")
			}
			if len(strings.Split(s.Signer, ";")) > 1 {
				if s.SigningRule != "" && s.SigningRule != AllOfSigningRequired && s.SigningRule != AnyOfSigningRequired {
					return fmt.Errorf("unknown signing rule: %s", s.SigningRule)
				}
			}
		default:
			return fmt.Errorf("unknown signer type: %s", s.Type)
		}
	case SignatureTypeUKEP:
		switch s.Type {
		case SignerTypeUser:
			if s.Signer == "" {
				return errors.New("signer is empty")
			}
		case SignerTypeGroup:
			if s.SignerGroupID == "" && s.SignerGroupIDPath == "" {
				return errors.New("signer group id is empty")
			}
			if s.SigningRule != "" && s.SigningRule != AllOfSigningRequired && s.SigningRule != AnyOfSigningRequired {
				return fmt.Errorf("unknown signing rule: %s", s.SigningRule)
			}
		case SignerTypeFromSchema:
			if s.Signer == "" {
				return errors.New("signer is empty")
			}
			if len(strings.Split(s.Signer, ";")) > 1 {
				if s.SigningRule != "" && s.SigningRule != AllOfSigningRequired && s.SigningRule != AnyOfSigningRequired {
					return fmt.Errorf("unknown signing rule: %s", s.SigningRule)
				}
			}
		default:
			return fmt.Errorf("unknown signer type: %s", s.Type)
		}
		//if s.SignatureCarrier == "" {
		//	return errors.New("no signature carrier provided")
		//}
		//carrier := s.SignatureCarrier
		//if carrier != SignatureCarrierCloud && carrier != SignatureCarrierToken && carrier != SignatureCarrierAll {
		//	return fmt.Errorf("unknown signature carrier: %s", s.SignatureCarrier)
		//}
	default:
		return fmt.Errorf("unknown signature type: %s", s.SignatureType)
	}

	return nil
}
