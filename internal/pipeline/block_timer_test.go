package pipeline

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUnmarshalTimerState(t *testing.T) {
	tests := []struct {
		name         string
		rawState     []byte
		wantDuration time.Duration
		wantStarted  bool
		wantExpired  bool
		wantErr      bool
	}{
		{
			name:         "old case",
			rawState:     []byte("{\"Expired\": true, \"Started\": false,\"Duration\": 700}"),
			wantErr:      false,
			wantExpired:  true,
			wantStarted:  false,
			wantDuration: 700,
		},
		{
			name:         "new case",
			rawState:     []byte("{\"expired\": true, \"started\": true,\"duration\": 60000000000}"),
			wantErr:      false,
			wantExpired:  true,
			wantStarted:  true,
			wantDuration: 60000000000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var td TimerData
			err := json.Unmarshal(tt.rawState, &td)
			if (err != nil) != tt.wantErr {
				t.Errorf("error: %v", err)
			}
			assert.Equal(t, tt.wantExpired, td.Expired)
			assert.Equal(t, tt.wantStarted, td.Started)
			assert.Equal(t, tt.wantDuration, td.Duration)
		})
	}
}
