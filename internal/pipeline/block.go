package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"gitlab.services.mts.ru/erius/pipeliner/internal/store"
	"io/ioutil"
	"net/http"

	"go.opencensus.io/trace"
)

type FunctionBlock struct {
	Name           string
	FunctionName   string
	FunctionInput  map[string]string
	FunctionOutput map[string]string
	NextStep       string
	runURL         string
}

func (fb FunctionBlock) Inputs() map[string]string {
	return fb.FunctionInput
}

func (fb FunctionBlock) Outputs() map[string]string {
	return fb.FunctionOutput
}

func (fb FunctionBlock) IsScenario() bool {
	return false
}

func (fb *FunctionBlock) Run(ctx context.Context, runCtx *store.VariableStore) error {
	_, s := trace.StartSpan(ctx, "run_function_block")
	defer s.End()

	runCtx.AddStep(fb.Name)

	values := make(map[string]interface{})

	for ikey, gkey := range fb.FunctionInput {
		val, ok := runCtx.GetValue(gkey) // if no value - empty value
		if ok {
			values[ikey] = val
		}
	}

	url := fmt.Sprintf(fb.runURL, fb.FunctionName)
	fmt.Println(url)

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
	fmt.Println(string(body))
	result := make(map[string]interface{})

	err = json.Unmarshal(body, &result)
	if err != nil {
		return err
	}

	for ikey, gkey := range fb.FunctionOutput {
		val := result[ikey]
		runCtx.SetValue(gkey, val)
	}

	return nil
}

func (fb *FunctionBlock) Next() string {
	return fb.NextStep
}
