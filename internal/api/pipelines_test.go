package api

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/test"
)

func Test_execVersion(t *testing.T) {
	pipeliner := APIEnv{
		DB:                   test.NewMockDB(),
		Remedy:               "",
		FaaS:                 "",
		SchedulerClient:      nil,
		NetworkMonitorClient: nil,
		HTTPClient:           nil,
		Statistic:            nil,
	}

	t.Run("name", func(t *testing.T) {

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*100)
		defer cancel()

		b, err := os.ReadFile("testdata/put_version_extra.json")
		assert.NoError(t, err)
		var p entity.EriusScenario
		err = json.Unmarshal(b, &p)
		assert.NoError(t, err)

		reqId := "123"

		vars := map[string]interface{}{}

		userName := "242"

		if _, _, err := pipeliner.execVersionInternal(ctx, &execVersionInternalDTO{
			reqID:         reqId,
			p:             &p,
			vars:          vars,
			syncExecution: true,
			userName:      userName,
		}); err != nil {
			assert.NoError(t, err)
		}
	})

}
