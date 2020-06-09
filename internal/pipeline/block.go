package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"io/ioutil"
	"net/http"
)

type FunctionBlock struct {
	BlockName      string
	FunctionName   string
	FunctionInput  map[string]string
	FunctionOutput map[string]string
	NextStep       string
	runURL	   string
}

func (fb *FunctionBlock) Run(ctx context.Context, store *VariableStore) error {
	ctx, s := trace.StartSpan(ctx, "run_function_block")
	defer s.End()
	values := make(map[string]interface{})
	for ikey, gkey := range fb.FunctionInput {
		fmt.Println(ikey, gkey)
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
	fmt.Println(string(b))
	fmt.Println(url)
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
		val, _ := result[ikey]
		store.SetValue(gkey, val)
	}
	return nil
}

func (fb *FunctionBlock) Next() string {
	return fb.NextStep
}

func NewFunction(name string, content map[string]interface{}) (*FunctionBlock, error) {
	fname, ok := content["name"].(string)
	if !ok {
		fname = ""
	}
	next, ok := content["next"].(string)
	if !ok {
		next = ""
	}
	inputs, ok := content["input"].([]interface{})
	if !ok {
		return nil, errors.New("invalid input format")
	}
	finput, err := createFuncParams(inputs)
	if err != nil {
		return nil, errors.Errorf("invalid input format: %s", err.Error())
	}
	outputs, ok := content["output"].([]interface{})
	foutput, err := createFuncParams(outputs)
	if err != nil {
		return nil, errors.Errorf("invalid output format: %s", err.Error())
	}
	fb := FunctionBlock{
		BlockName:      name,
		FunctionName:   fname,
		FunctionInput:  finput,
		FunctionOutput: foutput,
		NextStep:       next,
	}
	return &fb, nil
}

func createFuncParams(inp []interface{}) (map[string]string, error) {
	out := make(map[string]string)
	for _, v := range inp {
		inputParams, ok := v.(map[string]interface{})
		if !ok {
			return nil, errors.Errorf("can't convert %v to map", v)
		}
		varN, ok := inputParams["name"]
		if !ok {
			return nil, errors.New("can't get variable name")
		}
		varGN, ok := inputParams["global"]
		if !ok {
			return nil, errors.New("can't get variable global name")
		}
		varName, ok := varN.(string)
		if !ok {
			return nil, errors.Errorf("can't convert %v to string", varN)
		}
		varGlobalName, ok := varGN.(string)
		if !ok {
			return nil, errors.Errorf("can't convert %v to string", varGN)
		}
		out[varName] = varGlobalName
	}
	return out, nil
}
