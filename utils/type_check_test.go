package utils

import (
	"testing"
)

type ParamMetadata struct {
	Type        string
	Description string
	Items       *ParamMetadata
	Properties  map[string]ParamMetadata
}

func (p ParamMetadata) GetType() string {
	return p.Type
}

func (p ParamMetadata) GetProperties() map[string]interface{} {
	return nil
}

func TestSimpleTypeHandler(t *testing.T) {

	tests := []struct {
		Name          string
		variable      interface{}
		originalValue ParamMetadata
		WantErr       bool
		WantValue     interface{}
	}{
		{
			Name:     "integerType",
			variable: float64(10),
			originalValue: ParamMetadata{
				Type: "integer",
			},
			WantErr:   false,
			WantValue: int(10),
		},
		{
			Name:     "floatType",
			variable: float64(10),
			originalValue: ParamMetadata{
				Type: "number",
			},
			WantErr:   false,
			WantValue: float64(10),
		},
		{
			Name:     "integerWrongTypeFloat",
			variable: float64(10.6),
			originalValue: ParamMetadata{
				Type: "integer",
			},
			WantErr:   true,
			WantValue: float64(10.6),
		},
		{
			Name:     "integerWrongTypeString",
			variable: "here",
			originalValue: ParamMetadata{
				Type: "integer",
			},
			WantErr:   true,
			WantValue: "here",
		},
		{
			Name:     "String",
			variable: "here",
			originalValue: ParamMetadata{
				Type: "string",
			},
			WantErr:   false,
			WantValue: "here",
		},
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			err := simpleTypeHandler(&tt.variable, tt.originalValue)
			if (err == nil) == tt.WantErr {
				t.Errorf("unexpected error, %s", tt.Name)
			}
			if tt.variable != tt.WantValue {
				t.Errorf("unexpected type, %s", tt.Name)
			}
		})
	}
}
