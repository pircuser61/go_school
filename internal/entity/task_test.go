package entity

import (
	"encoding/json"
	"testing"
)

func TestAttachment_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		json    []byte
		want    Attachment
		wantErr bool
	}{
		{
			name: "full valid",
			json: []byte("{\"file_id\": \"file\", \"external_link\": \"link\"}"),
			want: Attachment{
				FileID:       "file",
				ExternalLink: "link",
			},
			wantErr: false,
		},
		{
			name: "from string id valid",
			json: []byte("\"94cfb0a9-bed4-490e-8a6c-dbd59da5701d\""),
			want: Attachment{
				FileID:       "94cfb0a9-bed4-490e-8a6c-dbd59da5701d",
				ExternalLink: "",
			},
			wantErr: false,
		},
		{
			name:    "from string not valid",
			json:    []byte("\"text\""),
			want:    Attachment{},
			wantErr: true,
		},
		{
			name:    "not valid",
			json:    []byte("\"function_mapping\":{\"system_name\":\"test_approve_group\"}"),
			want:    Attachment{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Attachment{}
			err := json.Unmarshal(tt.json, &got)
			if (err != nil) != tt.wantErr {
				t.Errorf("Attachment_UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Attachment_UnmarshalJSON() got = %v, want %v", got, tt.want)
			}
		})
	}
}
