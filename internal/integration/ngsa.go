package integration

import (
	"context"
	"fmt"
	"github.com/google/uuid"
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
	severn := 1
	source := "ERIUS"
	eventType := "EriusEvent"
	cause := "test message"
	addInf := "test message info"
	addTxt := "test message text"
	specProb := "test message problem specific"
	notID := notification + "__" + action
	usertext := "тестовый текст от пользователя"
	moInstance := "test instance"
	moClass := "test class"
	if action == actionLock {
		id := uuid.New()
		err := db.ActiveAlertNGSA(ctx, ns.db, id, severn, source, eventType,
			cause, addInf, addTxt, bts, specProb, notID, usertext, moInstance, moClass)
		if err != nil {
			return err
		}
		if reason != LockSuccessful {
			id := uuid.New()
			err := db.ActiveAlertNGSA(ctx, ns.db, id, severn, source, eventType,
				cause, addInf, addTxt, bts, specProb, notID, usertext, moInstance, moClass)
			if err != nil {
				return err
			}
			time.Sleep(3 * time.Minute)
			err = db.ClearAlertNGSA(ctx, ns.db, id)
			if err != nil {
				return err
			}
		}
	} else {
		id := uuid.New()
		err := db.ActiveAlertNGSA(ctx, ns.db, id, severn, source, eventType,
			cause, addInf, addTxt, bts, specProb, notID, usertext, moInstance, moClass)
		if err != nil {
			return err
		}
		if reason == UnlockSuccessful {
			name := notification + "__LOCK"
			linkedID, err := db.GetLingedAlertFromNGSA(ctx, ns.db, name)
			if err != nil {
				return err
			}
			err = db.ClearAlertNGSA(ctx, ns.db, linkedID)
			if err != nil {
				return err
			}
		}
		time.Sleep(3 * time.Minute)
		err = db.ClearAlertNGSA(ctx, ns.db, id)
		if err != nil {
			return err
		}

	}
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
			{
				Name:    "perceivedSeverity",
				Type:    script.TypeNumber,
				Comment: "",
			},
			{
				Name:    "source",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "eventTime",
				Type:    script.TypeString,
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
				Name:    "additionalText",
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
				Name:    "notificationIdentifier",
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
