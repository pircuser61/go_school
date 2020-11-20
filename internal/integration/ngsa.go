package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.opencensus.io/trace"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/erius/pipeliner/internal/db"
	"gitlab.services.mts.ru/erius/pipeliner/internal/metrics"
	"gitlab.services.mts.ru/erius/pipeliner/internal/script"
	"gitlab.services.mts.ru/erius/pipeliner/internal/store"
)

type NGSASend struct {
	Name      string
	db        db.Database
	NextBlock string
	Input     map[string]string
}

type NGSASendModel struct {
	State                  string `json:"state,omitempty"`
	AdditionalText         string `json:"additionalText,omitempty"`
	PerceivedSevernity     int    `json:"perceivedSeverity,omitempty"`
	MOIdentifier           string `json:"moIdentifier,omitempty"`
	NotificationIdentifier string `json:"notificationIdentifier,omitempty"`
	ManagedObjectInstance  string `json:"managedobjectinstance,omitempty"`
	ManagedObjectClass     string `json:"managedobjectclass,omitempty"`
	SpecificProblem        string `json:"specificProblem,omitempty"`
	UserText               string `json:"userText,omitempty"`
	ProbableCause          string `json:"probableCause,omitempty"`
	AdditionalInformation  string `json:"additionInformation,omitempty"`
	EventType              string `json:"eventType,omitempty"`
	TimeOut                int    `json:"timeout,omitempty"`
}

const (
	active = "ACTIVE"
	clear  = "CLEAR"
	erius  = "Erius"
)

func NewNGSASendIntegration(database db.Database) NGSASend {
	return NGSASend{
		db:    database,
		Input: make(map[string]string),
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
	return ns.DebugRun(ctx, runCtx)
}

//nolint:gocyclo //need bigger cyclomatic
func (ns NGSASend) DebugRun(ctx context.Context, runCtx *store.VariableStore) error {
	ctx, s := trace.StartSpan(ctx, "run_ngsa_send")
	defer s.End()

	monChan := make(chan bool)

	go func() {
		select {
		case ok := <-monChan:
			if ok {
				metrics.Stats.NGSAPushes.Ok.SetToCurrentTime()
			} else {
				metrics.Stats.NGSAPushes.Fail.SetToCurrentTime()
			}
		}

		close(monChan)

		errPush := metrics.Pusher.Push()
		if errPush != nil {
			fmt.Printf("can't push: %s\n", errPush.Error())
		}
	}()

	runCtx.AddStep(ns.Name)

	vals := make(map[string]interface{})

	inputs := ns.Model().Inputs
	for _, input := range inputs {
		v, okV := runCtx.GetValue(ns.Input[input.Name])
		if !okV {
			continue
		}

		vals[input.Name] = v
	}

	b, err := json.Marshal(vals)
	if err != nil {
		monChan <- false
		return err
	}

	m := NGSASendModel{}

	err = json.Unmarshal(b, &m)
	if err != nil {
		monChan <- false
		return err
	}

	if m.State != active && m.State != clear {
		monChan <- false
		return errors.New("unknown status")
	}

	if m.NotificationIdentifier == "" {
		monChan <- false
		return errors.New("notification id not found")
	}

	if m.TimeOut != 0 {
		go func() {
			time.Sleep(time.Duration(m.TimeOut) * time.Minute)

			var errActive error

			if m.State == active {
				errActive = ns.db.ActiveAlertNGSA(ctx, m.PerceivedSevernity,
					m.State, erius, m.EventType, m.ProbableCause, m.AdditionalInformation, m.AdditionalText,
					m.MOIdentifier, m.SpecificProblem, m.NotificationIdentifier, m.UserText, m.ManagedObjectInstance,
					m.ManagedObjectClass)
				if errActive != nil {
					runCtx.AddError(err)
				}
			}

			errClear := ns.db.ClearAlertNGSA(ctx, m.NotificationIdentifier)
			if errClear != nil {
				runCtx.AddError(err)
			}

			if errClear != nil || errActive != nil {
				monChan <- false
			} else {
				monChan <- true
			}
		}()

		return nil
	}

	if m.State == active {
		errNGSA := ns.db.ActiveAlertNGSA(ctx, m.PerceivedSevernity,
			m.State, erius, m.EventType, m.ProbableCause, m.AdditionalInformation, m.AdditionalText,
			m.MOIdentifier, m.SpecificProblem, m.NotificationIdentifier, m.UserText, m.ManagedObjectInstance,
			m.ManagedObjectClass)

		monChan <- errNGSA == nil

		return err
	}

	err = ns.db.ClearAlertNGSA(ctx, m.NotificationIdentifier)

	monChan <- err == nil

	return err
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
				Name:    "state",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "additionalText",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "perceivedSeverity",
				Type:    script.TypeNumber,
				Comment: "",
			},
			{
				Name:    "moIdentifier",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "notificationIdentifier",
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
				Name:    "probableCause",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "additionInformation",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "eventType",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "timeout",
				Type:    script.TypeNumber,
				Comment: "",
			},
		},
		Outputs:   nil,
		NextFuncs: []string{script.Next},
		ShapeType: script.ShapeIntegration,
	}
}
