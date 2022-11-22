package script

import (
	"encoding/json"
)

type AuthorizationHeader struct{}

type BlockUpdateData struct {
	ByLogin    string
	Action     string
	Parameters json.RawMessage
}
