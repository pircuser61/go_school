package script

import (
	"encoding/json"
	"net/url"

	"github.com/google/uuid"

	"github.com/iancoleman/orderedmap"
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

func OrderedMapToMap(om orderedmap.OrderedMap) (map[string]interface{}, error) {
	data, err := om.MarshalJSON()
	if err != nil {
		return nil, err
	}

	var m map[string]interface{}

	err = json.Unmarshal(data, &m)
	if err != nil {
		return nil, err
	}

	return m, nil
}

func MapToOrderedMap(m map[string]interface{}) (orderedmap.OrderedMap, error) {
	om := orderedmap.New()

	data, err := json.Marshal(m)
	if err != nil {
		return orderedmap.OrderedMap{}, err
	}

	err = json.Unmarshal(data, &om)
	if err != nil {
		return orderedmap.OrderedMap{}, err
	}

	return *om, nil
}

type URL struct {
	*url.URL
}

func (u *URL) UnmarshalYAML(unmarshal func(interface{}) error) error {
	stringURL := ""

	err := unmarshal(&stringURL)
	if err != nil {
		return err
	}

	u.URL, err = url.Parse(stringURL)

	return err
}
