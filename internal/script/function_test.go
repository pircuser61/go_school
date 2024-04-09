package script

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const versionExample = "916ad995-8d13-49fb-82ee-edd4f97649e2"

func TestExecutableFunctionParams_Validate(t *testing.T) {
	type fields struct {
		Name          string
		Version       string
		Mapping       JSONSchemaProperties
		Function      FunctionParam
		SLA           int
		NeedRetry     bool
		RetryCount    int
		RetryInterval int
		RetryPolicy   FunctionRetryPolicy
	}

	tests := []struct {
		name    string
		fields  fields
		wantErr error
	}{
		{
			name: "Tests of method ValidateSchemas, success case",
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
				Function: FunctionParam{
					Options: `{\"type\": \"sync\"}`,
				},
				SLA: int(60*time.Minute.Seconds() + 59*time.Second.Seconds()),
			},
			wantErr: nil,
		},
		{
			name: "Tests of method ValidateSchemas, missing type case",
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
			name: "Tests of method ValidateSchemas, missing items case",
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
			name: "Tests of method ValidateSchemas, missing properties case",
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
			name: "invalid sync function SLA",
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
				Function: FunctionParam{
					Options: `{\"type\": \"sync\"}`,
				},
				SLA: int(60*time.Minute.Seconds() + 60*time.Second.Seconds()),
			},
			wantErr: errors.New("sync function SLA is too long"),
		},
		{
			name: "invalid async function SLA",
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
				Function: FunctionParam{
					Options: `{\"type\": \"async\"}`,
				},
				SLA: int(365*24*time.Hour.Seconds() + 24*time.Hour.Seconds() + 59*time.Minute.Seconds()),
			},
			wantErr: errors.New("async function SLA is too long"),
		},
		{
			name: "Tests of method ValidateSchemas, missing name case",
			fields: fields{
				Name:    "",
				Version: versionExample,
				Mapping: JSONSchemaProperties{},
			},
			wantErr: errors.New("got no function name or version"),
		},
		{
			name: "Tests missing Retry count",
			fields: fields{
				Name:    "test",
				Version: versionExample,
				Mapping: JSONSchemaProperties{
					"param1": {
						Description: "param1 name",
						Type:        "string",
						Value:       "form_0.a",
					},
				},
				Function: FunctionParam{
					Options: `{\"type\": \"async\"}`,
				},
				SLA:           int(59 * time.Second.Seconds()),
				NeedRetry:     true,
				RetryInterval: 1,
				RetryPolicy:   "simple",
			},

			wantErr: errors.New("invalid retry count: 0"),
		},
		{
			name: "Tests missing retry interval",
			fields: fields{
				Name:    "test",
				Version: versionExample,
				Mapping: JSONSchemaProperties{
					"param1": {
						Description: "param1 name",
						Type:        "string",
						Value:       "form_0.a",
					},
				},
				Function: FunctionParam{
					Options: `{\"type\": \"async\"}`,
				},
				SLA:         int(59 * time.Second.Seconds()),
				NeedRetry:   true,
				RetryCount:  1,
				RetryPolicy: "simple",
			},

			wantErr: errors.New("invalid return interval: 0"),
		},
		{
			name: "Tests missing retry policy",
			fields: fields{
				Name:    "test",
				Version: versionExample,
				Mapping: JSONSchemaProperties{
					"param1": {
						Description: "param1 name",
						Type:        "string",
						Value:       "form_0.a",
					},
				},
				Function: FunctionParam{
					Options: `{\"type\": \"async\"}`,
				},
				SLA:           int(59 * time.Second.Seconds()),
				NeedRetry:     true,
				RetryCount:    1,
				RetryInterval: 1,
			},

			wantErr: errors.New("retry policy is empty"),
		},
		{
			name: "Tests invalid retry policy",
			fields: fields{
				Name:    "test",
				Version: versionExample,
				Mapping: JSONSchemaProperties{
					"param1": {
						Description: "param1 name",
						Type:        "string",
						Value:       "form_0.a",
					},
				},
				Function: FunctionParam{
					Options: `{\"type\": \"async\"}`,
				},
				SLA:           int(59 * time.Second.Seconds()),
				NeedRetry:     true,
				RetryCount:    1,
				RetryInterval: 1,
				RetryPolicy:   "testing policy",
			},

			wantErr: errors.New("invalid retry policy: testing policy"),
		},
		{
			name: "Tests right retry param",
			fields: fields{
				Name:    "test",
				Version: versionExample,
				Mapping: JSONSchemaProperties{
					"param1": {
						Description: "param1 name",
						Type:        "string",
						Value:       "form_0.a",
					},
				},
				Function: FunctionParam{
					Options: `{\"type\": \"async\"}`,
				},
				SLA:           int(59 * time.Second.Seconds()),
				NeedRetry:     true,
				RetryCount:    1,
				RetryInterval: 1,
				RetryPolicy:   "fibonacci",
			},

			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &ExecutableFunctionParams{
				Name:          tt.fields.Name,
				Version:       tt.fields.Version,
				Mapping:       tt.fields.Mapping,
				Function:      tt.fields.Function,
				SLA:           tt.fields.SLA,
				NeedRetry:     tt.fields.NeedRetry,
				RetryCount:    tt.fields.RetryCount,
				RetryInterval: tt.fields.RetryInterval,
				RetryPolicy:   tt.fields.RetryPolicy,
			}

			err := a.Validate()
			assert.Equal(t, tt.wantErr, err,
				fmt.Sprintf("Incorrect result. ValidateSchemas() method. Expect error %v, got %v", tt.wantErr, err))
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
			tt.wantErr(t, js.Validate(), "ValidateSchemas()")
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
			tt.wantErr(t, tt.properties.Validate(), "ValidateSchemas()")
		})
	}
}

//nolint:goconst //не нужно здесь нам чекать константы
func Test_functionTime_UnmarshalJSON(t *testing.T) {
	const (
		date1 = "2023-06-21 06:26:10.447720 +0000 UTC"
		date2 = "2023-06-21 06:26:10.44772 +0000 UTC"
		date3 = "2009-08-12T22:15:09.988"
	)

	type args struct {
		b []byte
	}

	tests := []struct {
		name    string
		ft      functionTime
		args    args
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "success: " + date1,
			ft:   functionTime{},
			args: args{
				b: []byte(date1),
			},
			wantErr: assert.NoError,
		},
		{
			name: "success: " + date2,
			ft:   functionTime{},
			args: args{
				b: []byte(date2),
			},
			wantErr: assert.NoError,
		},
		{
			name: "success: " + date3,
			ft:   functionTime{},
			args: args{
				b: []byte(date3),
			},
			wantErr: assert.NoError,
		},
		{
			name: "fail parse date",
			ft:   functionTime{},
			args: args{
				b: []byte("GGG"),
			},
			wantErr: assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantErr(t, tt.ft.UnmarshalJSON(tt.args.b), fmt.Sprintf("UnmarshalJSON(%v)", tt.args.b))
		})
	}
}

func Test_GetMappingFromInput(t *testing.T) {
	type fields struct {
		Input    string
		Mapping  JSONSchemaProperties
		Function FunctionParam
	}

	tests := []struct {
		name   string
		fields fields
		want   JSONSchemaProperties
	}{
		{
			name: "Tests of method GetMappingFromInput, same result",
			fields: fields{
				Input: `{
						"param1":{"description":"param1 name","type":"string"},
						"param2":{"description":"param2 name","type":"boolean"},
						"param3":{"description":"param3 name","type":"object", "properties": {
							"param3.1":{"description":"param3.1 name","type":"string","format":"date-time"},
							"param3.2":{"description":"param3.2 name","type":"array","items":{"type":"number"}}
						}}
						}`,
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
						Description: "param3 name",
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
			want: JSONSchemaProperties{
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
					Description: "param3 name",
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
		{
			name: "Tests of method GetMappingFromInput, + new field result",
			fields: fields{
				Input: `{
						"param1":{"description":"param1 name","type":"string"},
						"param2":{"description":"param2 name","type":"boolean"},
						"param3":{"description":"param3 name","type":"object", "properties": {
							"param3.1":{"description":"param3.1 name","type":"string","format":"date-time"},
							"param3.2":{"description":"param3.2 name","type":"array","items":{"type":"number"}}
						}},
						"param4":{"description":"param4 name","type":"integer"}
						}`,
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
						Description: "param3 name",
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
			want: JSONSchemaProperties{
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
					Description: "param3 name",
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
				"param4": {
					Description: "param4 name",
					Type:        "integer",
				},
			},
		},
		{
			name: "Tests of method GetMappingFromInput, - old field result",
			fields: fields{
				Input: `{
								"param1":{"description":"param1 name","type":"string"},
								"param3":{"description":"param3 name","type":"object", "properties": {
									"param3.1":{"description":"param3.1 name","type":"string","format":"date-time"}
								}}
								}`,
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
						Description: "param3 name",
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
			want: JSONSchemaProperties{
				"param1": {
					Description: "param1 name",
					Type:        "string",
					Value:       "form_0.a",
				},
				"param3": {
					Description: "param3 name",
					Type:        "object",
					Properties: JSONSchemaProperties{
						"param3.1": {
							Description: "param3.1 name",
							Type:        "string",
							Format:      "date-time",
							Value:       "form_0.c",
						},
					},
				},
			},
		},
		{
			name: "Tests of method GetMappingFromInput, change type object properties - simple",
			fields: fields{
				Input: `{
								"param1":{"description":"param1 name","type":"string"},
								"param2":{"description":"param2 name","type":"boolean"},
								"param3":{"description":"param3 name","type":"object", "properties": {
									"param3.1":{"description":"param3.1 name","type":"number","format":"date-time"},
									"param3.2":{"description":"param3.2 name","type":"array","items":{"type":"number"}}
								}}
								}`,
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
						Description: "param3 name",
						Type:        "object",
						Value:       "form_0.c",
						Properties: JSONSchemaProperties{
							"param3.1": {
								Description: "param3.1 name",
								Type:        "string",
								Format:      "date-time",
							},
							"param3.2": {
								Description: "param3.2 name",
								Type:        "array",
								Items: &ArrayItems{
									Type: "number",
								},
							},
						},
					},
				},
			},
			want: JSONSchemaProperties{
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
					Description: "param3 name",
					Type:        "object",
					Properties: JSONSchemaProperties{
						"param3.1": {
							Description: "param3.1 name",
							Type:        "number",
							Format:      "date-time",
						},
						"param3.2": {
							Description: "param3.2 name",
							Type:        "array",
							Items: &ArrayItems{
								Type: "number",
							},
						},
					},
				},
			},
		},
		{
			name: "Tests of method GetMappingFromInput, change object properties - array simple",
			fields: fields{
				Input: `{
								"param1":{"description":"param1 name","type":"string"},
								"param2":{"description":"param2 name","type":"boolean"},
								"param3":{"description":"param3 name","type":"object", "properties": {
									"param3.1":{"description":"param3.1 name","type":"string","format":"date-time"},
									"param3.2":{"description":"param3.2 name","type":"array","items":{"type":"string"}}
								}}
								}`,
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
						Description: "param3 name",
						Type:        "object",
						Value:       "form_0.c",
						Properties: JSONSchemaProperties{
							"param3.1": {
								Description: "param3.1 name",
								Type:        "string",
								Format:      "date-time",
							},
							"param3.2": {
								Description: "param3.2 name",
								Type:        "array",
								Items: &ArrayItems{
									Type: "number",
								},
							},
						},
					},
				},
			},
			want: JSONSchemaProperties{
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
					Description: "param3 name",
					Type:        "object",
					Properties: JSONSchemaProperties{
						"param3.1": {
							Description: "param3.1 name",
							Type:        "string",
							Format:      "date-time",
						},
						"param3.2": {
							Description: "param3.2 name",
							Type:        "array",
							Items: &ArrayItems{
								Type: "string",
							},
						},
					},
				},
			},
		},
		{
			name: "Tests of method GetMappingFromInput, change object properties - array object",
			fields: fields{
				Input: `{
								"param1":{"description":"param1 name","type":"string"},
								"param2":{"description":"param2 name","type":"boolean"},
								"param3":{"description":"param3 name","type":"object", "properties": {
									"param3.1":{"description":"param3.1 name","type":"string","format":"date-time"},
									"param3.2":{"description":"param3.2 name","type":"array",
										"items":{"type":"object","properties": {
											"param4.1":{"description":"param4.1 name","type":"string","format":"date-time"},
											"param4.2":{"description":"param4.2 name","type":"array","items":{"type":"string"}}
										}}}
									}}
								}`,
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
						Description: "param3 name",
						Type:        "object",
						Value:       "form_0.c",
						Properties: JSONSchemaProperties{
							"param3.1": {
								Description: "param3.1 name",
								Type:        "string",
								Format:      "date-time",
							},
							"param3.2": {
								Description: "param3.2 name",
								Type:        "array",
								Items: &ArrayItems{
									Type: "object",
									Properties: JSONSchemaProperties{
										"param4.1": {
											Description: "param4.1 name",
											Type:        "string",
											Format:      "date-time",
										},
										"param4.2": {
											Description: "param4.2 name",
											Type:        "array",
											Items: &ArrayItems{
												Type: "number",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: JSONSchemaProperties{
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
					Description: "param3 name",
					Type:        "object",
					Properties: JSONSchemaProperties{
						"param3.1": {
							Description: "param3.1 name",
							Type:        "string",
							Format:      "date-time",
						},
						"param3.2": {
							Description: "param3.2 name",
							Type:        "array",
							Items: &ArrayItems{
								Type: "object",
								Properties: JSONSchemaProperties{
									"param4.1": {
										Description: "param4.1 name",
										Type:        "string",
										Format:      "date-time",
									},
									"param4.2": {
										Description: "param4.2 name",
										Type:        "array",
										Items: &ArrayItems{
											Type: "string",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := &ExecutableFunctionParams{
				Mapping: tt.fields.Mapping,
				Function: FunctionParam{
					Input: tt.fields.Input,
				},
			}
			newMapping, err := params.GetMappingFromInput()
			assert.Nil(t, err)
			assert.Equal(t, tt.want, newMapping,
				fmt.Sprintf("Incorrect result. GetMappingFromInput() method. Expect result %v, got %v", tt.want, newMapping))

		})
	}
}

func Test_GetMappingFromInputRequired(t *testing.T) {
	type fields struct {
		Input    string
		Mapping  JSONSchemaProperties
		Function FunctionParam
	}

	tests := []struct {
		name      string
		fields    fields
		wantError bool
	}{
		{
			name: "Tests of method GetMappingFromInput, need required",
			fields: fields{
				Input: `{
						"param1":{"description":"param1 name","type":"string"},
						"param2":{"description":"param2 name","type":"boolean"},
						"param3":{"description":"param3 name","type":"object", "required": ["param3.2"], "properties": {
							"param3.1":{"description":"param3.1 name","type":"string","format":"date-time"},
							"param3.2":{"description":"param3.2 name","type":"array","items":{"type":"number"}}													
						 }}
						}`,
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
						Description: "param3 name",
						Type:        "object",
						Properties: JSONSchemaProperties{
							"param3.1": {
								Description: "param3.1 name",
								Type:        "string",
								Format:      "date-time",
								Value:       "form_0.c",
							},
						},
					},
				},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := &ExecutableFunctionParams{
				Mapping: tt.fields.Mapping,
				Function: FunctionParam{
					Input: tt.fields.Input,
				},
			}
			_, err := params.GetMappingFromInput()
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.Nil(t, err)
			}

		})
	}
}
