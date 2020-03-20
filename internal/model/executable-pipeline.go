package model

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"gitlab.services.mts.ru/erius/pipeliner/internal/dbconn"
)

const (
	kind = "kind"
	faas = "faas"
	stringsEqual = "strings_equal"
	gorun
)

type ExecutablePipeline struct {
	ContextStorage dbconn.PGConnection
	Entrypoint     string
	NowOnPoint     string
	Context        Context
	Blocks         map[string]Runner
	NextStep       string
}

func (ep *ExecutablePipeline) Run(ctx *Context) error {
	if ep.NowOnPoint == "" {
		ep.NowOnPoint = ep.Entrypoint
	}
	for ep.NowOnPoint != "" {
		err := ep.Blocks[ep.NowOnPoint].Run(&ep.Context)
		if err != nil {
			return errors.Errorf("error while executing pipeline on step %s: %w", ep.NowOnPoint, err)
		}
		ep.NowOnPoint = ep.Blocks[ep.NowOnPoint].Next()
	}
	return nil
}

func (ep *ExecutablePipeline) Next() string {
	return ep.NextStep
}

func (ep *ExecutablePipeline) UnmarshalJSON(b []byte) error {
	p := make(map[string]interface{})
	err := json.Unmarshal(b, &p)
	if err != nil {
		return err
	}
	pipeline := ExecutablePipeline{}
	for k, v := range p {

		switch v.(type) {
		case string:
			switch k {
			case "entrypoint":
				pipeline.Entrypoint = v.(string)
			}
		case map[string]interface{}:
			switch k {
			case "blocks":
				blocks, err := UnmarshalBlocks(v.(map[string]interface{}))
				fmt.Println(blocks, err)
			}
		}
	}
	return nil
}

func UnmarshalBlocks(m map[string]interface{}) (map[string]Runner, error) {
	blocks :=  make(map[string]Runner)
	for k, v := range m {
		b, ok := v.(map[string]interface{})
		if !ok {
			return nil, errors.New("can't parse block k")
		}
		switch b[kind] {
		case faas:
			block := NewFunction(k, b)
			fmt.Println(block)
		}
	}
	return blocks, nil
}
