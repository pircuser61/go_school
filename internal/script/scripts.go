package script

import (
	"context"
	"encoding/json"
	"fmt"
	"go.opencensus.io/trace"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
)

const (
	shapeFunction int = iota
	shapeRhombus
	shapeScenario
	shapeIntegration

	onTrue       string = "OnTrue"
	onFalse      string = "OnFalse"
	next         string = "Next"
	checkVarName string = "check"

	firstStringName string = "first_string"
	secondStringName string = "second_string"
	isEqualName string = "is_equal"

	typeBool   string = "bool"
	typeString string = "string"
	typeInt    string = "int"

	functionDeployed string = "deployed"

	TypeIF       = "term"
	TypePython   = "python3"
	TypeInternal = "internal"

	IconFunction     = "X24function"
	IconTerms        = "X24terms"
	IconIntegrations = "X24external"
	IconScenario     = "X24scenario"
)

type FunctionModel struct {
	BlockType string               `json:"block_type"`
	Title     string               `json:"title"`
	Inputs    []FunctionValueModel `json:"inputs"`
	Outputs   []FunctionValueModel `json:"outputs"`
	ShapeType int                  `json:"shape_type"`
	NextFuncs []string             `json:"next_funcs"`
}

type FunctionValueModel struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Comment string `json:"comment"`
}

type ShapeModel struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Color string `json:"color"`
	Icon  string `json:"icon"`
}

type ScriptManagerResponse struct {
	Function []SMFunc `json:"function"`
}

type SMFunc struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Template string `json:"template"`
	RefName  string `json:"ref_name"`
	Comment  string `json:"comment"`
	Input    struct {
		Fields []FunctionValueModel `json:"fields"`
	} `json:"input"`
	Output struct {
		Fields []FunctionValueModel `json:"fields"`
	} `json:"output"`
	Status string `json:"status"`
	Tags []string `json:"tags"`
}

func GetReadyFuncs(ctx context.Context, scriptManager string) ([]FunctionModel, error) {
	_, s := trace.StartSpan(context.Background(), "get_ready_modules")
	defer s.End()
	u, err := url.Parse(scriptManager)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "" {
		u.Scheme = "http"
	}
	u.Path = path.Join(u.Path, "/api/manager/function/list")

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	b, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return nil, err
	}

	fmt.Println(string(b))
	smf := ScriptManagerResponse{}
	err = json.Unmarshal(b, &smf)
	if err != nil {
		return nil, err
	}

	funcs := make([]FunctionModel, 0)
	ifstate := FunctionModel{
		BlockType: TypeIF,
		Title:     "if",
		Inputs: []FunctionValueModel{
			{
				Name: checkVarName,
				Type: typeBool,
			},
		},
		NextFuncs: []string{onTrue, onFalse},
		ShapeType: shapeRhombus,
	}

	testBlock := FunctionModel{
		BlockType: TypeInternal,
		Title:     "stings_is_equal",
		Inputs: []FunctionValueModel{
			{
				Name: firstStringName,
				Type: typeString,
			},
			{
				Name: secondStringName,
				Type: typeString,
			},
		},
		Outputs: []FunctionValueModel{
			{
				Name: isEqualName,
				Type: typeBool,
			},
		},
		NextFuncs: []string{next},
		ShapeType: shapeFunction,
	}
	funcs = append(funcs, ifstate, testBlock)
	for _, v := range smf.Function {
		if v.Status == functionDeployed {
			b := FunctionModel{
				BlockType: v.Template,
				Title:     v.Name,
				Inputs:    v.Input.Fields,
				Outputs:   v.Output.Fields,
				ShapeType: shapeFunction,
				NextFuncs: []string{next},
			}
			if b.Title == "cedar-test-1" {
				b.ShapeType = shapeIntegration
			}
			funcs = append(funcs, b)
		}
	}

	return funcs, nil
}

func GetShapes() ([]ShapeModel, error) {
	shapes := []ShapeModel{

		{
			ID:    shapeFunction,
			Title: IconFunction,
			Color: "#D31BB8",
			Icon:  IconFunction,
		},
		{
			ID:    shapeRhombus,
			Title: IconTerms,
			Color: "#1B6B54",
			Icon:  IconTerms,
		},
		{
			ID:    shapeIntegration,
			Title: IconIntegrations,
			Color: "#685C0F",
			Icon: IconIntegrations,
		},
		{
			ID:    shapeScenario,
			Title: IconScenario,
			Color: "#3F4568",
			Icon:  IconScenario,
		},
	}
	return shapes, nil
}
