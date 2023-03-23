package script

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

const versionExample = "916ad995-8d13-49fb-82ee-edd4f97649e2"
const jsonMappingString = `
	{
		"param1": {
			"description": "param1 name",
			"type":"string",
			"value":"form_0.a"
		},
		"param2": {
			"description": "param2 name",
			"type": "boolean",
			"value": "form_0.b"
		},
		"param3": {
			"description": "param4 name",
			"type": "object",
			"properties": {
				"param3.1": {
					"description": "param3.1 name",
					"type": "string",
					"format":"date-time",
					"value":"form_0.c"
				},
				"param3.2": {
					"description": "param3.2 name",
					"type": "array",
					"items": {
						"type":"number"
					},
					"value":"form_0.d"
				}
			}
		}
	}

`

func TestExecutableFunctionParams_Validate(t *testing.T) {
	type fields struct {
		Name    string
		Version string
		Mapping MappingParam
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
				Mapping: MappingParam{
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
						Properties: MappingParam{
							"param3.1": {
								Description: "param3.1 name",
								Type:        "string",
								Format:      "date-time",
								Value:       "form_0.c",
							},
							"param3.2": {
								Description: "param3.2 name",
								Type:        "array",
								Items: &MappingParamItems{
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
				Mapping: MappingParam{
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
						Properties: MappingParam{
							"param3.1": {
								Description: "param3.1 name",
								Type:        "string",
								Format:      "date-time",
								Value:       "form_0.c",
							},
							"param3.2": {
								Description: "param3.2 name",
								Type:        "",
								Items: &MappingParamItems{
									Type: "number",
								},
								Value: "form_0.d",
							},
						},
					},
				},
			},
			wantErr: errors.New("type and description are required"),
		},
		{
			name: "Tests of method Validate, missing items case",
			fields: fields{
				Name:    "executable_function_0",
				Version: versionExample,
				Mapping: MappingParam{
					"param3": {
						Description: "param4 name",
						Type:        "object",
						Properties: MappingParam{
							"param3.1": {
								Description: "param3.1 name",
								Type:        "string",
								Format:      "date-time",
								Value:       "form_0.c",
							},
							"param3.2": {
								Description: "param3.2 name",
								Type:        "array",
								Items: &MappingParamItems{
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
				Mapping: MappingParam{
					"param3": {
						Description: "param4 name",
						Type:        "object",
						Properties: MappingParam{
							"param3.1": {
								Description: "param3.1 name",
								Type:        "string",
								Format:      "date-time",
								Value:       "form_0.c",
							},
							"param3.2": {
								Description: "param3.2 name",
								Type:        "array",
								Items: &MappingParamItems{
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
				Mapping: MappingParam{},
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

func TestMappingParam_Validate(t *testing.T) {
	unmarshaledMappingParam := MappingParam{}
	err := json.Unmarshal([]byte(jsonMappingString), &unmarshaledMappingParam)
	assert.Nil(t, err)

	err = unmarshaledMappingParam.Validate()
	assert.Nil(t, err)
}
