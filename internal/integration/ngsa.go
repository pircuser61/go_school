package integration

import (
	"context"
	"fmt"
	"gitlab.services.mts.ru/erius/pipeliner/internal/dbconn"
	"gitlab.services.mts.ru/erius/pipeliner/internal/script"
	"gitlab.services.mts.ru/erius/pipeliner/internal/store"
	"go.opencensus.io/trace"
	"time"
)

type NGSASend struct {
	Name      string
	ttl       time.Duration
	db        *dbconn.PGConnection
	NextBlock string
	Input     map[string]string
}

func NewNGSASendIntegration(db *dbconn.PGConnection, ttl int, name string) NGSASend {
	return NGSASend{
		ttl: time.Duration(ttl) * time.Minute,
		db:  db,
	}
}

func (ns NGSASend) Run(ctx context.Context, runCtx *store.VariableStore) error {
	_, s := trace.StartSpan(ctx, "run_ngsa_send")
	defer s.End()

	runCtx.AddStep(ns.Name)
	notification, _ := runCtx.GetString(ns.Input["notification"])
	reason, _ := runCtx.GetString(ns.Input["reason"])
	action, _ := runCtx.GetString(ns.Input["action"])
	fmt.Println(notification, reason, action)
	return nil
}

func (ns NGSASend) Next() string {
	return ns.NextBlock
}

func (ns NGSASend) Model() script.FunctionModel {
	return script.FunctionModel{
		BlockType: script.TypeInternal,
		Title:     "ngsa-send-alarm",
		Inputs: []script.FunctionValueModel{
			{
				Name:    "notification",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "reason",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "action",
				Type:    script.TypeString,
				Comment: "",
			},
		},
		Outputs:   nil,
		NextFuncs: []string{script.Next},
		ShapeType: script.ShapeIntegration,
	}
}
