package model

import (
	"encoding/json"
	"github.com/google/uuid"
)

type Pipeline struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	Pipeline string    `json:"pipeline"`
	Context  Context
}

func NewPipeline(data []byte) (*Pipeline, error) {
	p := Pipeline{}
	err := json.Unmarshal(data, &p)
	if err != nil {
		return nil, err
	}
	p.Context = NewContext()
	return &p, nil
}

func (p *Pipeline) Run() error {
	return nil
}