package script

import (
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

const versionExample = "916ad995-8d13-49fb-82ee-edd4f97649e2"

func TestExecutableFunctionParams_Validate(t *testing.T) {
	type fields struct {
		Name    string
		Version string
		Mapping JSONSchemaProperties
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr error
	}{
		{
			name: "Tests of method Validate, success case",
			fields: fields{
				Name:    "executable_function_0",
				Version: versionExample,
				Mapping: JSONSchemaProperties{
					"param1": {
						Description: "param1 name",
						Type:        "string",
						Value:       "form_0.a",
					},
					"param2": {
						Description: "param2 name",
						Type:        "boolean",
						Value:       "form_0.b",
					},
					"param3": {
						Description: "param4 name",
						Type:        "object",
						Properties: JSONSchemaProperties{
							"param3.1": {
								Description: "param3.1 name",
								Type:        "string",
								Format:      "date-time",
								Value:       "form_0.c",
							},
							"param3.2": {
								Description: "param3.2 name",
								Type:        "array",
								Items: &ArrayItems{
									Type: "number",
								},
								Value: "form_0.d",
							},
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "Tests of method Validate, missing type case",
			fields: fields{
				Name:    "executable_function_0",
				Version: versionExample,
				Mapping: JSONSchemaProperties{
					"param1": {
						Description: "param1 name",
						Type:        "string",
						Value:       "form_0.a",
					},
					"param2": {
						Description: "param2 name",
						Type:        "boolean",
						Value:       "form_0.b",
					},
					"param3": {
						Description: "param4 name",
						Type:        "object",
						Properties: JSONSchemaProperties{
							"param3.1": {
								Description: "param3.1 name",
								Type:        "string",
								Format:      "date-time",
								Value:       "form_0.c",
							},
							"param3.2": {
								Description: "param3.2 name",
								Type:        "",
								Items: &ArrayItems{
									Type: "number",
								},
								Value: "form_0.d",
							},
						},
					},
				},
			},
			wantErr: errors.New("type is required"),
		},
		{
			name: "Tests of method Validate, missing items case",
			fields: fields{
				Name:    "executable_function_0",
				Version: versionExample,
				Mapping: JSONSchemaProperties{
					"param3": {
						Description: "param4 name",
						Type:        "object",
						Properties: JSONSchemaProperties{
							"param3.1": {
								Description: "param3.1 name",
								Type:        "string",
								Format:      "date-time",
								Value:       "form_0.c",
							},
							"param3.2": {
								Description: "param3.2 name",
								Type:        "array",
								Items: &ArrayItems{
									Type: "array",
								},
								Value: "form_0.d",
							},
						},
					},
				},
			},
			wantErr: errors.New("items is required"),
		},
		{
			name: "Tests of method Validate, missing properties case",
			fields: fields{
				Name:    "executable_function_0",
				Version: versionExample,
				Mapping: JSONSchemaProperties{
					"param3": {
						Description: "param4 name",
						Type:        "object",
						Properties: JSONSchemaProperties{
							"param3.1": {
								Description: "param3.1 name",
								Type:        "string",
								Format:      "date-time",
								Value:       "form_0.c",
							},
							"param3.2": {
								Description: "param3.2 name",
								Type:        "array",
								Items: &ArrayItems{
									Type: "object",
								},
								Value: "form_0.d",
							},
						},
					},
				},
			},
			wantErr: errors.New("properties is required"),
		},
		{
			name: "Tests of method Validate, missing name case",
			fields: fields{
				Name:    "",
				Version: versionExample,
				Mapping: JSONSchemaProperties{},
			},
			wantErr: errors.New("got no function name or version"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &ExecutableFunctionParams{
				Name:    tt.fields.Name,
				Version: tt.fields.Version,
				Mapping: tt.fields.Mapping,
			}

			err := a.Validate()
			assert.Equal(t, tt.wantErr, err,
				fmt.Sprintf("Incorrect result. Validate() method. Expect error %v, got %v", tt.wantErr, err))
		})
	}
}

func TestJSONSchema_Validate(t *testing.T) {
	type fields struct {
		Type       string
		Properties JSONSchemaProperties
		Required   []string
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "success case",
			fields: fields{
				Type: "object",
				Properties: JSONSchemaProperties{
					"param1": JSONSchemaPropertiesValue{
						Type:  "string",
						Value: "start_0.A",
					},
				},
				Required: []string{"param1"},
			},
			wantErr: assert.NoError,
		},
		{
			name: "wrong type, error case",
			fields: fields{
				Type: "string",
				Properties: JSONSchemaProperties{
					"param1": JSONSchemaPropertiesValue{
						Type:  "string",
						Value: "start_0.A",
					},
				},
				Required: []string{"param1"},
			},
			wantErr: assert.Error,
		},
		{
			name: "missing required property, error case",
			fields: fields{
				Type: "string",
				Properties: JSONSchemaProperties{
					"param2": JSONSchemaPropertiesValue{
						Type:  "string",
						Value: "start_0.A",
					},
				},
				Required: []string{"param1"},
			},
			wantErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			js := &JSONSchema{
				Type:       tt.fields.Type,
				Properties: tt.fields.Properties,
				Required:   tt.fields.Required,
			}
			tt.wantErr(t, js.Validate(), fmt.Sprintf("Validate()"))
		})
	}
}

func TestJSONSchemaProperties_Validate1(t *testing.T) {
	tests := []struct {
		name       string
		properties JSONSchemaProperties
		wantErr    assert.ErrorAssertionFunc
	}{
		{
			name: "success case",
			properties: map[string]JSONSchemaPropertiesValue{
				"param1": {
					Description: "param1 name",
					Type:        "string",
					Value:       "form_0.a",
				},
				"param2": {
					Description: "param2 name",
					Type:        "boolean",
					Value:       "form_0.b",
				},
				"param3": {
					Description: "param4 name",
					Type:        "object",
					Properties: JSONSchemaProperties{
						"param3-1": {
							Description: "param3-1 name",
							Type:        "string",
							Format:      "date-time",
							Value:       "form_0.c",
						},
						"param3-2": {
							Description: "param3-2 name",
							Type:        "array",
							Items: &ArrayItems{
								Type: "number",
							},
							Value: "form_0.d",
						},
					},
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "error case",
			properties: map[string]JSONSchemaPropertiesValue{
				"param1": {
					Description: "param1 name",
					Type:        "object",
					Properties: JSONSchemaProperties{
						"param2": {
							Type: "object",
							Properties: JSONSchemaProperties{
								"param1": {
									Description: "param3-1 name",
									Type:        "string",
									Format:      "date-time",
									Value:       "form_0.a",
								},
							},
						},
					},
					Value: "form_0.e",
				},
			},
			wantErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantErr(t, tt.properties.Validate(), fmt.Sprintf("Validate()"))
		})
	}
}
