package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gitlab.services.mts.ru/erius/pipeliner/internal/metrics"

	"gitlab.services.mts.ru/erius/pipeliner/internal/db"
	"gitlab.services.mts.ru/erius/pipeliner/internal/script"
	"gitlab.services.mts.ru/erius/pipeliner/internal/store"
	"go.opencensus.io/trace"
)

type NGSASend struct {
	Name      string
	ttl       time.Duration
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

func NewNGSASendIntegration(database db.Database, ttl int, name string) NGSASend {
	return NGSASend{
		ttl:   time.Duration(ttl) * time.Minute,
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
	err := ns.DebugRun(ctx, runCtx)
	if err != nil {
		metrics.Stats.NGSAPushes.Fail.SetToCurrentTime()
	} else {
		metrics.Stats.NGSAPushes.Ok.SetToCurrentTime()
	}

	errPush := metrics.Pusher.Push()
	if errPush != nil {
		fmt.Printf("can't push: %s\n", errPush.Error())
	}

	return err
}

func (ns NGSASend) DebugRun(ctx context.Context, runCtx *store.VariableStore) error {
	ctx, s := trace.StartSpan(ctx, "run_ngsa_send")
	defer s.End()

	runCtx.AddStep(ns.Name)

	vals := make(map[string]interface{})

	inputs := ns.Model().Inputs
	for _, input := range inputs {
		v, ok := runCtx.GetValue(ns.Input[input.Name])
		if !ok {
			continue
		}

		vals[input.Name] = v
	}

	b, err := json.Marshal(vals)
	if err != nil {
		return err
	}

	m := NGSASendModel{}

	err = json.Unmarshal(b, &m)
	if err != nil {
		return err
	}

	if m.State != active && m.State != clear {
		return errors.New("unknown status")
	}

	if m.NotificationIdentifier == "" {
		return errors.New("notification id not found")
	}

	if m.TimeOut != 0 {
		go func() {
			time.Sleep(time.Duration(m.TimeOut) * time.Minute)

			if m.State == active {
				err = ns.db.ActiveAlertNGSA(ctx, m.PerceivedSevernity,
					m.State, erius, m.EventType, m.ProbableCause, m.AdditionalInformation, m.AdditionalText,
					m.MOIdentifier, m.SpecificProblem, m.NotificationIdentifier, m.UserText, m.ManagedObjectInstance,
					m.ManagedObjectClass)
				if err != nil {
					runCtx.AddError(err)
				}
			}

			err = ns.db.ClearAlertNGSA(ctx, m.NotificationIdentifier)
			if err != nil {
				runCtx.AddError(err)
			}
		}()

		return nil
	}

	if m.State == active {
		return ns.db.ActiveAlertNGSA(ctx, m.PerceivedSevernity,
			m.State, erius, m.EventType, m.ProbableCause, m.AdditionalInformation, m.AdditionalText,
			m.MOIdentifier, m.SpecificProblem, m.NotificationIdentifier, m.UserText, m.ManagedObjectInstance,
			m.ManagedObjectClass)
	}

	return ns.db.ClearAlertNGSA(ctx, m.NotificationIdentifier)
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
