package integration

import (
	"context"
	"encoding/json"
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
}

var (
	LockDenied       = "Автоматическая блокировка не требуется"
	LockSuccessful   = "Блокировка Успешна"
	UnlockSuccessful = "Разблокировка Успешна"

	actionLock = "LOCK"
)

func NewNGSASendIntegration(db *dbconn.PGConnection, ttl int, name string) NGSASend {
	return NGSASend{
		ttl:   time.Duration(ttl) * time.Minute,
		db:    db,
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
	ctx, s := trace.StartSpan(ctx, "run_ngsa_send")
	defer s.End()
	runCtx.AddStep(ns.Name)
	vals := make(map[string]interface{})
	inputs := ns.Model().Inputs
	for _, input := range inputs {
		fmt.Println(ns.Input[input.Name])
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
	fmt.Println("input Data:", string(b))
	m := NGSASendModel{}
	err = json.Unmarshal(b, &m)
	if err != nil {
		return err
	}
	fmt.Printf("after unmarshal: %+v\n", m)
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
		},
		Outputs:   nil,
		NextFuncs: []string{script.Next},
		ShapeType: script.ShapeIntegration,
	}
}
