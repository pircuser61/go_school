package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"go.opencensus.io/trace"
)

type FunctionBlock struct {
	Name           BlockName
	FunctionName   string
	FunctionInput  map[string]string
	FunctionOutput map[string]string
	NextStep       BlockName
	runURL         string
}

func (fb *FunctionBlock) Run(ctx context.Context, store *VariableStore) error {
	_, s := trace.StartSpan(ctx, "run_function_block")
	defer s.End()

	store.AddStep(fb.Name)

	values := make(map[string]interface{})

	for ikey, gkey := range fb.FunctionInput {
		val, ok := store.GetValue(gkey) // if no value - empty value
		if ok {
			values[ikey] = val
		}
	}

	url := fmt.Sprintf(fb.runURL, fb.FunctionName)

	b, err := json.Marshal(values)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(b))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	result := make(map[string]interface{})

	err = json.Unmarshal(body, &result)
	if err != nil {
		return err
	}

	for ikey, gkey := range fb.FunctionOutput {
		val := result[ikey]
		store.SetValue(gkey, val)
	}

	return nil
}

func (fb *FunctionBlock) Next() BlockName {
	return fb.NextStep
}
