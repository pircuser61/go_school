package script

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFillFormMapWithConstants(t *testing.T) {
	type args struct {
		constants map[string]interface{}
		mapData   map[string]interface{}
	}

	mapData := map[string]interface{}{
		"uuid-1-field-1": 1,
		"uuid-2-field-2": 2,
		"uuid-3-field-3": 3,
	}

	wantSuccess := map[string]interface{}{
		"uuid-1-field-1": 1,
		"uuid-2-field-2": 2,
		"uuid-3-field-3": 3,
		"const_1":        "const value 1",
		"const_2":        "const value 2",
		"const_3":        "const value 3",
	}

	tests := []struct {
		name    string
		args    args
		want    map[string]interface{}
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "Success",
			args: args{
				constants: map[string]interface{}{
					"form.const_1":       "const value 1",
					"form.uuid1.const_2": "const value 2",
					"const_3":            "const value 3",
				},
				mapData: mapData,
			},
			want:    wantSuccess,
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantErr(t, FillFormMapWithConstants(
				tt.args.constants, tt.args.mapData),
				fmt.Sprintf("FillFormMapWithConstants(%v, %v)",
					tt.args.constants,
					tt.args.mapData,
				),
			)
		})

		if tt.want != nil {
			if !assert.ObjectsAreEqual(tt.want, mapData) {
				assert.Equalf(t, tt.want, mapData, "result map in not correct")
			}
		}
	}
}
