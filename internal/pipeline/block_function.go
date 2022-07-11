package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

const (
	ErrorKey     = "error"
	KeyDelimiter = "."
)

type FunctionBlock struct {
	Name           string
	Type           string
	FunctionName   string
	FunctionInput  map[string]string
	FunctionOutput map[string]string
	Nexts          map[string][]string
	RunURL         string
}

func (fb *FunctionBlock) GetStatus() Status {
	return StatusFinished
}

func (fb *FunctionBlock) GetTaskHumanStatus() TaskHumanStatus {
	return ""
}

func (fb *FunctionBlock) GetType() string {
	return fb.Type
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

func (fb *FunctionBlock) DebugRun(ctx context.Context, _ *stepCtx, runCtx *store.VariableStore) error {
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

	b, err := json.Marshal(values)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(b))
	if err != nil {
		return err
	}

	// fixme extract "X-Request-Id" to variable

	if xReqID, ok := ctx.Value("X-Request-Id").(string); ok {
		req.Header.Set("X-Request-Id", xReqID)
	}

	req.Header.Set("Content-ReqType", "application/json")

	const timeoutMinutes = 15

	client := &http.Client{
		Timeout: timeoutMinutes * time.Minute,
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if len(body) != 0 {
		result := make(map[string]interface{})

		err = json.Unmarshal(body, &result)
		if err != nil {
			return err
		}

		if val, ok := result[ErrorKey].(string); ok {
			funcError := errors.New(val)
			runCtx.AddError(funcError)
			runCtx.SetValue(fb.Name+KeyDelimiter+ErrorKey, val)
		}

		for ikey, gkey := range fb.FunctionOutput {
			val := result[ikey]
			runCtx.SetValue(gkey, val)
		}
	}

	return nil
}

func (fb *FunctionBlock) Next(runCtx *store.VariableStore) ([]string, bool) {
	nexts, ok := fb.Nexts[DefaultSocket]
	if !ok {
		return nil, false
	}
	return nexts, true
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

	url := fmt.Sprintf(fb.RunURL, fb.FunctionName)

	b, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(b))
	if err != nil {
		return nil, err
	}

	// fixme extract "X-Request-Id" to variable
	req.Header.Set("Content-ReqType", "application/json")

	const timeoutMinutes = 15

	client := &http.Client{
		Timeout: timeoutMinutes * time.Minute,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

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

func (fb *FunctionBlock) GetState() interface{} {
	return nil
}

func (fb *FunctionBlock) Update(_ context.Context, _ *script.BlockUpdateData) (interface{}, error) {
	return nil, nil
}
