package script

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"

	"github.com/google/uuid"

	"go.opencensus.io/trace"
)

type ShapeEntity struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Icon  string `json:"icon"`
}

type SMResponse struct {
	Function []SMFunctionEntity `json:"function"`
}

type SMFunctionEntity struct {
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
	Status string        `json:"status"`
	Tags   []FunctionTag `json:"tags"`
}

type FunctionTag struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	Color    string    `json:"color"`
	Approved bool      `json:"approved"`
}

func GetReadyFuncs(ctx context.Context, scriptManager string, httpClient *http.Client) (FunctionModels, error) {
	_, s := trace.StartSpan(ctx, "get_ready_modules")
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

	resp, err := httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	b, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()

	if err != nil {
		return nil, err
	}

	smf := SMResponse{}

	err = json.Unmarshal(b, &smf)
	if err != nil {
		return nil, err
	}

	funcs := make([]FunctionModel, 0)

	for i := range smf.Function {
		v := &smf.Function[i]
		if v.Status == functionDeployed {
			b := FunctionModel{
				BlockType: v.Template,
				Title:     v.Name,
				Inputs:    v.Input.Fields,
				Outputs:   v.Output.Fields,
				ShapeType: shapeFunction,
				NextFuncs: []string{Next},
			}

			if b.Title == "cedar-test-1" || b.Title == "get-no-energy-action" || b.Title == "send-ngsa" {
				b.ShapeType = ShapeIntegration
			}

			funcs = append(funcs, b)
		}
	}

	return funcs, nil
}

func GetShapes() ([]ShapeEntity, error) {
	shapes := []ShapeEntity{

		{
			ID:    shapeFunction,
			Title: IconFunction,
			Icon:  IconFunction,
		},
		{
			ID:    shapeRhombus,
			Title: IconTerms,
			Icon:  IconTerms,
		},
		{
			ID:    ShapeIntegration,
			Title: IconIntegrations,
			Icon:  IconIntegrations,
		},
		{
			ID:    ShapeScenario,
			Title: IconScenario,
			Icon:  IconScenario,
		},
		{
			ID:    shapeConnector,
			Title: IconConnector,
			Icon:  IconConnector,
		},
		{
			ID:    shapeVariable,
			Title: IconVariable,
			Icon:  IconVariable,
		},
	}

	return shapes, nil
}
