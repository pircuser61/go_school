package integration

import (
	"context"
	"gitlab.services.mts.ru/erius/pipeliner/internal/db"
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

var (
	LockDenied       = "Автоматическая блокировка не требуется"
	LockSuccessful   = "Блокировка Успешна"
	UnlockSuccessful = "Разблокировка Успешна"

	actionLock = "LOCK"
)

func NewNGSASendIntegration(db *dbconn.PGConnection, ttl int, name string) NGSASend {
	return NGSASend{
		ttl: time.Duration(ttl) * time.Minute,
		db:  db,
	}
}


func (ns NGSASend) Inputs() map[string]string {
	return ns.Input
}

func (ns NGSASend) Outputs() map[string]string {
	return make(map[string]string)
}

func (ns NGSASend) IsScenario() bool {
	return false
}

func (ns NGSASend) Run(ctx context.Context, runCtx *store.VariableStore) error {
	ctx, s := trace.StartSpan(ctx, "run_ngsa_send")
	defer s.End()
	runCtx.AddStep(ns.Name)
	notification, err := runCtx.GetString(ns.Input["notification"])
	if err != nil {
		return err
	}
	reason, err := runCtx.GetString(ns.Input["reason"])
	if err != nil {
		return err
	}
	action, err := runCtx.GetString(ns.Input["action"])
	bts, err := runCtx.GetString(ns.Input["moIdentifier"])
	if err != nil {
		return err
	}
	severn := 4
	sev, ok := runCtx.GetValue(ns.Input["perceivedSeverity"])
	if ok {
		severn, ok = sev.(int)
		if !ok {
			severn = 4
		}
	}
	notID := notification + "__" + action
	source := "Erius"
	eventType, err := runCtx.GetString(ns.Input["eventType"])
	if err != nil {
		eventType = "Environmental alarm"
	}
	cause, _ := runCtx.GetString(ns.Input["probableCause"])
	addInf, _ := runCtx.GetString(ns.Input["additionalInformation"])
	addTxt, _ := runCtx.GetString(ns.Input["additionalText"])
	specProb, _ := runCtx.GetString(ns.Input["specificProblem"])
	usertext, _ := runCtx.GetString(ns.Input["userText"])
	moInstance, _ := runCtx.GetString(ns.Input["managedobjectinstance"])
	moClass, _ := runCtx.GetString(ns.Input["managedobjectclass"])
	if action == actionLock {
		err := db.ActiveAlertNGSA(ctx, ns.db, severn, source, eventType,
			cause, addInf, addTxt, bts, specProb, notID, usertext, moInstance, moClass)
		if err != nil {
			return err
		}
		if reason != LockSuccessful {
			err := db.ActiveAlertNGSA(ctx, ns.db, severn, source, eventType,
				cause, addInf, addTxt, bts, specProb, notID, usertext, moInstance, moClass)
			if err != nil {
				return err
			}
			time.Sleep(3 * time.Minute)
			err = db.ClearAlertNGSA(ctx, ns.db, notID)
			if err != nil {
				return err
			}
		}
	} else {
		err := db.ActiveAlertNGSA(ctx, ns.db, severn, source, eventType,
			cause, addInf, addTxt, bts, specProb, notID, usertext, moInstance, moClass)
		if err != nil {
			return err
		}
		if reason == UnlockSuccessful {
			name := notification + "__LOCK"
			err = db.ClearAlertNGSA(ctx, ns.db, name)
			if err != nil {
				return err
			}
		}
		time.Sleep(3 * time.Minute)
		err = db.ClearAlertNGSA(ctx, ns.db, notification)
		if err != nil {
			return err
		}

	}

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
			{
				Name:    "perceivedSeverity",
				Type:    script.TypeNumber,
				Comment: "",
			},
			{
				Name:    "probableCause",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "additionalInformation",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "moIdentifier",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "specificProblem",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "userText",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "managedobjectinstance",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "managedobjectclass",
				Type:    script.TypeString,
				Comment: "",
			},
		},
		Outputs:   nil,
		NextFuncs: []string{script.Next},
		ShapeType: script.ShapeIntegration,
	}
}
