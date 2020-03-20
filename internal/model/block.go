package model

import (
	"errors"
	"fmt"
	"strings"
)

type FunctionBlock struct {
	BlockName      string
	ScriptServer   string
	FunctionName   string
	FunctionInput  Context
	FunctionOutput Context
	NextStep       string
}

func (fb *FunctionBlock) Run(ctx *Context) error {
	if fb.ScriptServer == "go-inside" {
		return fb.RunInGo(ctx)
	}
	return nil
}

func (fb *FunctionBlock) Next() string {
	return fb.NextStep
}

func (fb *FunctionBlock) RunInGo(ctx *Context) error {
	switch fb.FunctionName {
	case "parse":
		alert, ok := ctx.GetValue("alert").(string)
		if !ok {
			return errors.New("can't get alert")
		}
		alertKind, niossID, eventId := RunParse(alert)
		ctx.SetValue("kind", alertKind)
		ctx.SetValue("nioss_id", niossID)
		ctx.SetValue("event_id", eventId)
	case "connect":

	}
	return nil
}
func get(alertSlice []string, i int) string {
	if len(alertSlice) <= i {
		return ""
	}
	return alertSlice[i]
}

func RunParse(alert string) (string, string, string) {
	alertSlice := strings.Split(alert, "__")
	return get(alertSlice, 0), get(alertSlice, 1), get(alertSlice, 2)
}

func NewFunction(name string, content map[string]interface{}) FunctionBlock {
	fname, ok := content["name"].(string)
	if !ok {
		fname = ""
	}
	next, ok := content["next"].(string)
	if !ok {
		next = ""
	}
	finput := Context{}
	for k, v := range content["input"].(map[string]interface{}) {
		fmt.Println("  in   - ", k, v)
	}
	foutput := Context{}
	for k, v := range content["output"].(map[string]interface{}) {
		fmt.Println("  out  - ", k, v)
	}
	fb := FunctionBlock{
		BlockName:      name,
		FunctionName:   fname,
		FunctionInput:  finput,
		FunctionOutput: foutput,
		NextStep:       next,
	}
	return fb
}
