package script

import (
	"context"
	"encoding/json"
	"go.opencensus.io/trace"
	"net/http"
	"net/url"
	"path"
)

const (
	shapeRectangle int = iota
	shapeRhombus
	shapeCircle
	shapeTriangle
	shapeIntegration

	onTrue       string = "OnTrue"
	onFalse      string = "OnFalse"
	next         string = "Next"
	checkVarName string = "check"

	testVarNameString string = "teststring"
	testVarNameInt    string = "testint"

	typeBool   string = "bool"
	typeString string = "string"
	typeInt    string = "int"

	functionDeployed string = "deployed"
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

type ScriptManagerResponse []SMFunc

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
	u.Path = path.Join(u.Path, "/api/manager/faas/list")

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	smf := ScriptManagerResponse{}
	err = json.NewDecoder(resp.Body).Decode(&smf)
	if err != nil {
		return nil, err
	}

	funcs := make([]FunctionModel, 0)
	ifstate := FunctionModel{
		BlockType: "if-statement",
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
		BlockType: "testblock",
		Title:     "testblock",
		Inputs: []FunctionValueModel{
			{
				Name: testVarNameString,
				Type: typeString,
			},
		},
		Outputs: []FunctionValueModel{
			{
				Name: testVarNameInt,
				Type: typeInt,
			},
		},
		NextFuncs: []string{next},
		ShapeType: shapeRectangle,
	}
	funcs = append(funcs, ifstate, testBlock)
	for _, v := range smf {
		if v.Status == functionDeployed {
			b := FunctionModel{
				BlockType: v.Template,
				Title:     v.Name,
				Inputs:    v.Input.Fields,
				Outputs:   v.Output.Fields,
				ShapeType: shapeRectangle,
				NextFuncs: []string{next},
			}
			funcs = append(funcs, b)
		}
	}

	return funcs, nil
}

func GetShapes() ([]ShapeModel, error) {
	shapes := []ShapeModel{
		{
			ID:    shapeRectangle,
			Title: "rectangle",
			Color: "#123456",
			Icon:  "rectangle",
		},
		{
			ID:    shapeRhombus,
			Title: "rhombus",
			Color: "#7890AB",
			Icon:  "rhombus",
		},
		{
			ID:    shapeIntegration,
			Title: "integration",
			Color: "#CDEF12",
			Icon:  "integration",
		},
		{
			ID:    shapeCircle,
			Title: "circle",
			Color: "#345678",
			Icon:  "circle",
		},
		{
			ID:    shapeTriangle,
			Title: "triangle",
			Color: "#90ABCD",
			Icon:  "triangle",
		},
	}
	return shapes, nil
}
