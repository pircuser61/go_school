package db

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

func TestAddFieldsFilter(t *testing.T) {
	tests := []struct {
		name string
		fl   *entity.TaskFilter
		want string
	}{
		{
			name: "empty fields",
			fl: &entity.TaskFilter{
				GetTaskParams: entity.GetTaskParams{
					Fields: &[]string{""},
				},
			},
			want: "",
		},
		{
			name: "one fields (without answer)",
			fl: &entity.TaskFilter{
				GetTaskParams: entity.GetTaskParams{
					Fields: &[]string{"group-uuid-3"},
				},
			},
			want: "",
		},
		{
			name: "two fields (two - is answer)",
			fl: &entity.TaskFilter{
				GetTaskParams: entity.GetTaskParams{
					Fields: &[]string{"field-uuid-2.a"},
				},
			},
			want: " AND w.child_id IS NULL AND jsonb_path_exists((vs.content -> 'State' -> vs.step_name -> 'application_body')::jsonb, '$[*] ? (@.\"field-uuid-2\" == \"a\")')",
		},
		{
			name: "three fields (three - is answer)",
			fl: &entity.TaskFilter{
				GetTaskParams: entity.GetTaskParams{
					Fields: &[]string{"group-uuid-3.field-uuid-2.a"},
				},
			},
			want: " AND w.child_id IS NULL AND jsonb_path_exists((vs.content -> 'State' -> vs.step_name -> 'application_body')::jsonb, '$.\"group-uuid-3\"[*] ? (@.\"field-uuid-2\" == \"a\")')",
		},
		{
			name: "four fields (four - is answer)",
			fl: &entity.TaskFilter{
				GetTaskParams: entity.GetTaskParams{
					Fields: &[]string{"group-uuid-3.group-uuid-2.field-uuid-2.a"},
				},
			},
			want: " AND w.child_id IS NULL AND jsonb_path_exists((vs.content -> 'State' -> vs.step_name -> 'application_body')::jsonb, '$.\"group-uuid-3\".\"group-uuid-2\"[*] ? (@.\"field-uuid-2\" == \"a\")')",
		},
		{
			name: "five fields (five - is answer)",
			fl: &entity.TaskFilter{
				GetTaskParams: entity.GetTaskParams{
					Fields: &[]string{"group-uuid-3.group-uuid-2.group-uuid-1.field-uuid-2.a"},
				},
			},
			want: " AND w.child_id IS NULL AND jsonb_path_exists((vs.content -> 'State' -> vs.step_name -> 'application_body')::jsonb, '$.\"group-uuid-3\".\"group-uuid-2\".\"group-uuid-1\"[*] ? (@.\"field-uuid-2\" == \"a\")')",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := compileGetTaskQueryMaker{
				q: "",
			}

			c.addFieldsFilter(tt.fl)

			assert.Equalf(t, tt.want, c.q, "c.addFieldsFilter(%v)", tt.fl.Fields)
		})
	}
}
