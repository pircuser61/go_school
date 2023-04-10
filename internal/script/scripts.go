package script

import (
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

func OrderedMapToMap(om orderedmap.OrderedMap) map[string]interface{} {
	m := make(map[string]interface{})
	for _, key := range om.Keys() {
		value, _ := om.Get(key)
		m[key] = value
	}

	return m
}

func MapToOrderedMap(m map[string]interface{}) orderedmap.OrderedMap {
	om := orderedmap.New()
	for key, value := range m {
		om.Set(key, value)
	}

	return *om
}
