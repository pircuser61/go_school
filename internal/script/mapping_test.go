package script

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

const jsonInputProperties = `{
        "A": "A",
        "B": false,
        "C": {
            "C-1": "some string",
            "C-2": [
                1.0,
                2.0,
                3.5,
                4.7,
                0.4
            ],
            "C-3": {
                "param3-3-1": "another string"
            },
            "C-4": {
                "C-4-1": 1.0
            }
        }
    }`

const jsonOutputProperties = `{
        "param1": "A",
        "param2": false,
        "param3": {
            "param3-1": "some string",
            "param3-2": [
                1.0,
                2.0,
                3.5,
                4.7,
                0.4
            ],
            "param3-3": {
                "param3-3-1": "another string"
            },
            "param3-4": {
                "param3-4-1": 1.0
            }
        },
		"param4": {
			"param4-1": {
				"param4-1-1": {
					"param4-1-1-1": 5.2
				}
			}
		},
		"param5": "some string"
    }`

const jsonInputProperties2 = `{
        "param1": {
			"param1-1": "some string"
		}
	}`

func TestMapData(t *testing.T) {
	input := make(map[string]interface{})
	err := json.Unmarshal([]byte(jsonInputProperties), &input)
	assert.Nil(t, err)

	output := make(map[string]interface{})
	err = json.Unmarshal([]byte(jsonOutputProperties), &output)
	assert.Nil(t, err)

	input2 := make(map[string]interface{})
	err = json.Unmarshal([]byte(jsonInputProperties2), &input2)
	assert.Nil(t, err)

	type args struct {
		mapping     JSONSchemaProperties
		input       map[string]interface{}
		required    []string
		levelToRoot int
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]interface{}
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "success case",
			args: args{
				mapping: JSONSchemaProperties{
					"param1": {
						Title: "param1",
						Type:  "string",
						Value: "start_0.A",
					},
					"param2": {
						Title: "param2",
						Type:  "boolean",
						Value: "start_0.B",
					},
					"param3": {
						Title: "param3",
						Type:  "object",
						Properties: JSONSchemaProperties{
							"param3-1": {
								Type:  "string",
								Value: "start_0.C.C-1",
							},
							"param3-2": {
								Type: "array",
								Items: &ArrayItems{
									Type: "number",
								},
								Value: "start_0.C.C-2",
							},
							"param3-3": {
								Type: "object",
								Properties: JSONSchemaProperties{
									"param3-3-1": {
										Type: "string",
									},
								},
								Value: "start_0.C.C-3",
							},
							"param3-4": {
								Type: "object",
								Properties: JSONSchemaProperties{
									"param3-4-1": {
										Type:  "number",
										Value: "start_0.C.C-4.C-4-1",
									},
								},
							},
						},
					},
					"param4": {
						Type: object,
						Properties: JSONSchemaProperties{
							"param4-1": {
								Type: object,
								Properties: JSONSchemaProperties{
									"param4-1-1": {
										Type: object,
										Properties: JSONSchemaProperties{
											"param4-1-1-1": {
												Type:    "number",
												Default: 5.2,
												Value:   "start_0.path.to.variable",
											},
										},
									},
								},
							},
						},
					},
					"param5": {
						Type:    "string",
						Default: "some string",
					},
				},
				input:       input,
				required:    []string{"param1"},
				levelToRoot: 1,
			},
			want:    output,
			wantErr: assert.NoError,
		},
		{
			name: "missing required variable, error case",
			args: args{
				mapping: JSONSchemaProperties{
					"param1": {
						Type: "string",
					},
				},
				input:       nil,
				required:    []string{"param1"},
				levelToRoot: 1,
			},
			wantErr: assert.Error,
		},
		{
			name: "invalid path to variable, error case",
			args: args{
				mapping: JSONSchemaProperties{
					"param1": {
						Type: object,
						Properties: JSONSchemaProperties{
							"param1-1": {
								Type:  "string",
								Value: "start_0.param1.param1-1.param1-1-1",
							},
						},
					},
				},
				input:       input2,
				required:    nil,
				levelToRoot: 1,
			},
			wantErr: assert.Error,
		},
		{
			name: "invalid path to root/variable, error case",
			args: args{
				mapping: JSONSchemaProperties{
					"param1": {
						Type: object,
						Properties: JSONSchemaProperties{
							"param1-1": {
								Type: "string",
							},
						},
						Value: "param1",
					},
				},
				input:       input2,
				required:    nil,
				levelToRoot: 1,
			},
			wantErr: assert.Error,
		},
		{
			name: "invalid json, error case",
			args: args{
				mapping: JSONSchemaProperties{
					"param1": {
						Type: object,
						Properties: JSONSchemaProperties{
							"param1-1": {
								Type: "number",
							},
						},
						Value: "servicedesk_application_0.application_body.param1",
					},
				},
				input:       input2,
				required:    nil,
				levelToRoot: 2,
			},
			wantErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MapData(tt.args.mapping, tt.args.input, tt.args.required, tt.args.levelToRoot)
			if !tt.wantErr(t, err, fmt.Sprintf("MapData(%v, %v, %v, %v)", tt.args.mapping, tt.args.input, tt.args.required, tt.args.levelToRoot)) {
				return
			}
			assert.Equalf(t, tt.want, got, "MapData(%v, %v, %v, %v)", tt.args.mapping, tt.args.input, tt.args.required, tt.args.levelToRoot)
		})
	}
}
