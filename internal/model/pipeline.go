package model

import (
	"encoding/json"
	"github.com/google/uuid"
)

type Pipeline struct {
	ID            uuid.UUID          `json:"id,omitempty"`
	Name          string             `json:"name"`
	Pipeline      ExecutablePipeline `json:"pipeline"`
	FunctionInput map[string]interface{}
}

func NewPipeline(data []byte) (*Pipeline, error) {
	p := Pipeline{}
	err := json.Unmarshal(data, &p)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (p *Pipeline) Run(ctx Context) error {
	return nil
}
