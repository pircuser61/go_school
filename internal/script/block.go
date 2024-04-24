package script

import (
	"encoding/json"
)

type BlockInputsValidator interface {
	Validate() error
}

type AuthorizationHeader struct{}

type BlockUpdateData struct {
	ByLogin    string
	Action     string
	Parameters json.RawMessage
}
