package entity

import (
	"github.com/google/uuid"
	"testing"
	"time"
)

func TestEriusScenario_Validate(t *testing.T) {
	type fields struct {
		ID        uuid.UUID
		VersionID uuid.UUID
		Status    int
		HasDraft  bool
		Name      string
		Input     []EriusFunctionValue
		Output    []EriusFunctionValue
		Pipeline  struct {
			Entrypoint string               `json:"entrypoint"`
			Blocks     map[string]EriusFunc `json:"blocks"`
		}
		ProcessSettings ProcessSettings
		CreatedAt       *time.Time
		ApprovedAt      *time.Time
		Author          string
		Tags            []EriusTagInfo
		Comment         string
		CommentRejected string
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "success case",
			fields: fields{
				ProcessSettings: ProcessSettings{
					StartSchema: "{\"param1\": {\"type\": \"string\", \"description\": \"param1 name \"}," +
						"\"param2\": {\"type\": \"boolean\", \"description\": \"param2 name\"}}",
					EndSchema: "{\"param3\":{\"description\":\"param3 name\",\"type\":\"integer\"}}",
					ExternalSystems: []ExternalSystem{
						{
							Id: uuid.MustParse("ee957ac8-4e2b-4324-bf43-c48f10c51596"),
							InputSchema: "{\"param1\": {\"type\": \"string\", \"description\": \"param1 name \"}," +
								"\"param2\": {\"type\": \"boolean\", \"description\": \"param2 name\"}}",
							OutputSchema: "{\"param3\":{\"description\":\"param3 name\",\"type\":\"integer\"}}",
						},
						{
							Id:           uuid.MustParse("6a9da7c3-8541-4170-82fe-629051daecec"),
							InputSchema:  "{\"param4\":{\"type\":\"string\"},\"param5\":{\"type\":\"boolean\"}}",
							OutputSchema: "{\"param6\":{\"type\":\"integer\"}}",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "duplicate id, error",
			fields: fields{
				ProcessSettings: ProcessSettings{
					ExternalSystems: []ExternalSystem{
						{
							Id: uuid.MustParse("ee957ac8-4e2b-4324-bf43-c48f10c51596"),
							InputSchema: "{\"param1\": {\"type\": \"string\", \"description\": \"param1 name \"}," +
								"\"param2\": {\"type\": \"boolean\", \"description\": \"param2 name\"}}",
							OutputSchema: "{\"param3\":{\"description\":\"param3 name\",\"type\":\"integer\"}}",
						},
						{
							Id:           uuid.MustParse("ee957ac8-4e2b-4324-bf43-c48f10c51596"),
							InputSchema:  "{\"param4\":{\"type\":\"string\"},\"param5\":{\"type\":\"boolean\"}}",
							OutputSchema: "{\"param6\":{\"type\":\"integer\"}}",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "duplicate id, error",
			fields: fields{
				ProcessSettings: ProcessSettings{
					ExternalSystems: []ExternalSystem{
						{
							Id:           uuid.MustParse("ee957ac8-4e2b-4324-bf43-c48f10c51596"),
							InputSchema:  "{invalid json}",
							OutputSchema: "{\"param3\":{\"description\":\"param3 name\",\"type\":\"integer\"}}",
						},
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := EriusScenario{
				ID:              tt.fields.ID,
				VersionID:       tt.fields.VersionID,
				Status:          tt.fields.Status,
				HasDraft:        tt.fields.HasDraft,
				Name:            tt.fields.Name,
				Input:           tt.fields.Input,
				Output:          tt.fields.Output,
				ProcessSettings: tt.fields.ProcessSettings,
				Pipeline:        tt.fields.Pipeline,
				CreatedAt:       tt.fields.CreatedAt,
				ApprovedAt:      tt.fields.ApprovedAt,
				Author:          tt.fields.Author,
				Tags:            tt.fields.Tags,
				Comment:         tt.fields.Comment,
				CommentRejected: tt.fields.CommentRejected,
			}
			if err := s.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
