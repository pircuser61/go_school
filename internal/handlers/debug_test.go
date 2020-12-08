package handlers

import (
	"reflect"
	"testing"
)

func Test_sliceToMap(t *testing.T) {
	tests := []struct {
		name  string
		items []string
		want  map[string]struct{}
	}{
		{
			name:  "if empty slice return empty map",
			items: nil,
			want:  map[string]struct{}{},
		},
		{
			name:  "ok",
			items: []string{"1", "2", "3"},
			want: map[string]struct{}{
				"1": {},
				"2": {},
				"3": {},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sliceToMap(tt.items); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("sliceToMap() = %v, want %v", got, tt.want)
			}
		})
	}
}
