package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"gitlab.services.mts.ru/erius/pipeliner/internal/store"

	"go.opencensus.io/trace"
)

type FunctionBlock struct {
	Name           string
	FunctionName   string
	FunctionInput  map[string]string
	FunctionOutput map[string]string
	NextStep       string
	RunURL         string
}

func (fb *FunctionBlock) Inputs() map[string]string {
	return fb.FunctionInput
}

func (fb *FunctionBlock) Outputs() map[string]string {
	return fb.FunctionOutput
}

func (fb *FunctionBlock) IsScenario() bool {
	return false
}

// WBMARK - Implement
func (fb *FunctionBlock) Run(ctx context.Context, runCtx *store.VariableStore) error {
	return fb.DebugRun(ctx, runCtx)
}

func (fb *FunctionBlock) DebugRun(ctx context.Context, runCtx *store.VariableStore) error {
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

	url := fmt.Sprintf(fb.RunURL, fb.FunctionName)
	fmt.Println(url)

	b, err := json.Marshal(values)
	if err != nil {
		return err
	}

	fmt.Println(string(b))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(b))
	if err != nil {
		return err
	}

	// fixme extract "X-Request-Id" to variable

	if xReqID, ok := ctx.Value("X-Request-Id").(string); ok {
		req.Header.Set("X-Request-Id", xReqID)
	}

	req.Header.Set("Content-Type", "application/json")

	const timeoutMinutes = 15

	client := &http.Client{
		Timeout: timeoutMinutes * time.Minute,
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	fmt.Println("response:", string(body))

	if len(body) != 0 {
		result := make(map[string]interface{})

		err = json.Unmarshal(body, &result)
		if err != nil {
			return err
		}

		for ikey, gkey := range fb.FunctionOutput {
			val := result[ikey]
			runCtx.SetValue(gkey, val)
		}
	}

	return nil
}

func (fb *FunctionBlock) Next(runCtx *store.VariableStore) (string, bool) {
	return fb.NextStep, true
}

func (fb *FunctionBlock) NextSteps() []string {
	nextSteps := []string{fb.NextStep}

	return nextSteps
}

func (fb *FunctionBlock) RunOnly(ctx context.Context, runCtx *store.VariableStore) (interface{}, error) {
	_, s := trace.StartSpan(ctx, "run_function_block")
	defer s.End()

	values := make(map[string]interface{})

	for ikey, gkey := range fb.FunctionInput {
		val, ok := runCtx.GetValue(gkey) // if no value - empty value
		if ok {
			values[ikey] = val
		}
	}

	fmt.Println(values, fb.FunctionInput)

	url := fmt.Sprintf(fb.RunURL, fb.FunctionName)
	fmt.Println(url)

	b, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}

	fmt.Println(string(b))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(b))
	if err != nil {
		return nil, err
	}

	// fixme extract "X-Request-Id" to variable
	req.Header.Set("Content-Type", "application/json")

	const timeoutMinutes = 15

	client := &http.Client{
		Timeout: timeoutMinutes * time.Minute,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	fmt.Println(string(body))

	if len(body) != 0 {
		result := make(map[string]interface{})
		err = json.Unmarshal(body, &result)

		if err != nil {
			return string(body), nil
		}

		return result, nil
	}

	return string(body), nil
}
