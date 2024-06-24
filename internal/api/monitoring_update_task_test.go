package api

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.services.mts.ru/abp/myosotis/logger"
)

func TestIsTypeCorrect(t *testing.T) {
	log := logger.GetLogger(context.TODO())

	testJSON := `
{
    "test_integer": 12,
    "test_float": 12.1,
    "test_string":"some string",
    "test_bool":true,
    "test_array": [1,2],
    "test_object": {
        "type":"string",
        "value":"value"
    },
	"test_null": null
}
`
	var testValues map[string]interface{}

	err := json.Unmarshal([]byte(testJSON), &testValues)
	if err != nil {
		log.Error(err)
		return
	}

	type args struct {
		t string
		v any
	}
	tests := []struct {
		name    string
		args    args
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name:    "valid string",
			args:    args{t: "string", v: testValues["test_string"]},
			wantErr: assert.NoError,
		},
		{
			name:    "valid integer",
			args:    args{t: "integer", v: testValues["test_integer"]},
			wantErr: assert.NoError,
		},
		{
			name:    "valid float",
			args:    args{t: "number", v: testValues["test_float"]},
			wantErr: assert.NoError,
		},
		{
			name:    "valid boolean",
			args:    args{t: "boolean", v: testValues["test_bool"]},
			wantErr: assert.NoError,
		},
		{
			name:    "valid array",
			args:    args{t: "array", v: testValues["test_array"]},
			wantErr: assert.NoError,
		},
		{
			name:    "valid object",
			args:    args{t: "object", v: testValues["test_object"]},
			wantErr: assert.NoError,
		},
		{
			name:    "invalid string",
			args:    args{t: "string", v: testValues["test_bool"]},
			wantErr: assert.Error,
		},
		{
			name:    "invalid integer",
			args:    args{t: "integer", v: testValues["test_float"]},
			wantErr: assert.Error,
		},
		{
			name:    "invalid float",
			args:    args{t: "number", v: testValues["test_object"]},
			wantErr: assert.Error,
		},
		{
			name:    "invalid boolean",
			args:    args{t: "boolean", v: testValues["test_array"]},
			wantErr: assert.Error,
		},
		{
			name:    "invalid array",
			args:    args{t: "array", v: testValues["test_string"]},
			wantErr: assert.Error,
		},
		{
			name:    "invalid object",
			args:    args{t: "object", v: testValues["test_integer"]},
			wantErr: assert.Error,
		},
		{
			name:    "empty type",
			args:    args{t: "", v: testValues["test_integer"]},
			wantErr: assert.Error,
		},
		{
			name:    "null with empty type",
			args:    args{t: "", v: testValues["test_null"]},
			wantErr: assert.Error,
		},
		{
			name:    "Null with string type",
			args:    args{t: "string", v: testValues["test_null"]},
			wantErr: assert.Error,
		},
		{
			name:    "Null with object type",
			args:    args{t: "object", v: testValues["test_null"]},
			wantErr: assert.NoError,
		},
		{
			name:    "Null with array type",
			args:    args{t: "array", v: testValues["test_null"]},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantErr(t, IsTypeCorrect(tt.args.t, tt.args.v), fmt.Sprintf("IsTypeCorrect(%v, %v)", tt.args.t, tt.args.v))
		})
	}
}
