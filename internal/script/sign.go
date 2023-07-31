package script

import (
	"errors"
	"fmt"
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
)

type SignParams struct {
	Type             SignerType `json:"type"`
	SigningRule      `json:"approvementRule"`
	Signer           string           `json:"approver,omitempty"`
	SignatureType    SignatureType    `json:"signature_type"`
	SignatureCarrier SignatureCarrier `json:"signature_carrier,omitempty"`

	SignerGroupID     string `json:"signer_group_id,omitempty"`
	SignerGroupName   string `json:"signer_group_name,omitempty"`
	SignerGroupIDPath string `json:"signer_group_id_path,omitempty"`

	FormsAccessibility []FormAccessibility `json:"forms_accessibility"`
}

func (a *SignParams) Validate() error {
	switch a.SignatureType {
	case SignatureTypePEP:
		if a.Type != SignerTypeUser {
			return fmt.Errorf("bad signer type: %s", a.Type)
		}
		if a.Signer == "" {
			return errors.New("signer is empty")
		}
	case SignatureTypeUNEP:
		switch a.Type {
		case SignerTypeUser:
			if a.Signer == "" {
				return errors.New("signer is empty")
			}
		case SignerTypeGroup:
			if a.SignerGroupID == "" && a.SignerGroupIDPath == "" {
				return errors.New("signer group id is empty")
			}
			if a.SigningRule != "" && a.SigningRule != AllOfSigningRequired && a.SigningRule != AnyOfSigningRequired {
				return fmt.Errorf("unknown signing rule: %s", a.SigningRule)
			}
		case SignerTypeFromSchema:
			if a.Signer == "" {
				return errors.New("signer is empty")
			}
		default:
			return fmt.Errorf("unknown signer type: %s", a.Type)
		}
	case SignatureTypeUKEP:
		switch a.Type {
		case SignerTypeUser:
			if a.Signer == "" {
				return errors.New("signer is empty")
			}
		case SignerTypeGroup:
			if a.SignerGroupID == "" && a.SignerGroupIDPath == "" {
				return errors.New("signer group id is empty")
			}
			if a.SigningRule != "" && a.SigningRule != AllOfSigningRequired && a.SigningRule != AnyOfSigningRequired {
				return fmt.Errorf("unknown signing rule: %s", a.SigningRule)
			}
		case SignerTypeFromSchema:
			if a.Signer == "" {
				return errors.New("signer is empty")
			}
		default:
			return fmt.Errorf("unknown signer type: %s", a.Type)
		}
		if a.SignatureCarrier == "" {
			return errors.New("no signature carrier provided")
		}
		carrier := a.SignatureCarrier
		if carrier != SignatureCarrierCloud && carrier != SignatureCarrierToken {
			return fmt.Errorf("unknown signature carrier: %s", a.SignatureCarrier)
		}
	default:
		return fmt.Errorf("unknown signature type: %s", a.SignatureType)
	}

	return nil
}
