package model

import (
	"context"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
)

type FunctionBlock struct {
	BlockName      string
	FunctionName   string
	FunctionInput  map[string]string
	FunctionOutput map[string]string
	NextStep       string
}

func (fb *FunctionBlock) Run(ctx context.Context, store *VariableStore) error {
	ctx, s := trace.StartSpan(ctx, "run_function_block")
	defer s.End()

	switch fb.FunctionName {
	case "parse":
		alert, err := store.GetStringWithInput(fb.FunctionInput, "alert")
		if err != nil {
			return err
		}

		k, niossid, eventid := RunParse(alert)
		err = store.SetStringWithOutput(fb.FunctionOutput, "kind", k)
		if err != nil {
			return err
		}
		err = store.SetStringWithOutput(fb.FunctionOutput, "nioss_id", niossid)
		if err != nil {
			return err
		}
		err = store.SetStringWithOutput(fb.FunctionOutput, "event_id", eventid)
		if err != nil {
			return err
		}
	case "connect":
		k, err := store.GetStringWithInput(fb.FunctionInput, "kind")
		if err != nil {
			return err
		}
		event, err := store.GetStringWithInput(fb.FunctionInput, "event_id")
		if err != nil {
			return err
		}
		nioss, err := store.GetStringWithInput(fb.FunctionInput, "nioss_id")
		if err != nil {
			return err
		}
		alert := RunConnect(event, k, nioss)
		err = store.SetStringWithOutput(fb.FunctionOutput, "alert", alert)
		if err != nil {
			return err
		}
	case "check_lock":
		event, err := store.GetStringWithInput(fb.FunctionInput, "event_id")
		if err != nil {
			return err
		}
		result := CheckLock(event)
		err = store.SetBoolWithOutput(fb.FunctionOutput, "lock", result)
		if err != nil {
			return err
		}
	case "check_unlock":
		event, err := store.GetStringWithInput(fb.FunctionInput, "event_id")
		if err != nil {
			return err
		}
		result := CheckUnlock(event)
		err = store.SetBoolWithOutput(fb.FunctionOutput, "unlock", result)
		if err != nil {
			return err
		}
	case "lock":
		nioss, err := store.GetStringWithInput(fb.FunctionInput, "nioss_id")
		if err != nil {
			return err
		}
		result := datastore.Lock(nioss)
		err = store.SetStringWithOutput(fb.FunctionOutput, "result", result)
		if err != nil {
			return err
		}
	case "unlock":
		nioss, err := store.GetStringWithInput(fb.FunctionInput, "nioss_id")
		if err != nil {
			return err
		}
		result := datastore.Unlock(nioss)
		err = store.SetStringWithOutput(fb.FunctionOutput, "result", result)
		if err != nil {
			return err
		}
	case "address":
		nioss, err := store.GetStringWithInput(fb.FunctionInput, "nioss_id")
		if err != nil {
			return err
		}
		result := datastore.Address(nioss)
		err = store.SetStringWithOutput(fb.FunctionOutput, "address", result)
		if err != nil {
			return err
		}
	}
	return nil
}

func (fb *FunctionBlock) Next() string {
	return fb.NextStep
}

func get(alertSlice []string, i int) string {
	if len(alertSlice) <= i {
		return ""
	}
	return alertSlice[i]
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
