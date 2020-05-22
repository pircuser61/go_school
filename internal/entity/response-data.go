package entity

import (
	"github.com/google/uuid"
	"gitlab.services.mts.ru/erius/pipeliner/internal/script"
	"time"
)

type EriusScenarioList struct {
	Pipelines []EriusScenarioInfo `json:"pipelines"`
	Drafts    []EriusScenarioInfo `json:"drafts"`
	OnApprove []EriusScenarioInfo `json:"on_approve"`
	Tags      []EriusTagInfo      `json:"tags"`
}

type EriusScenarioInfo struct {
	ID            uuid.UUID   `json:"id"`
	VersionID     uuid.UUID   `json:"version_id"`
	CreatedAt     time.Time   `json:"created_at"`
	ApprovedAt    time.Time   `json:"approved_at"`
	Author        string      `json:"author"`
	Approver      string      `json:"approver"`
	Name          string      `json:"name"`
	Tags          []time.Time `json:"tags"`
	LastRun       time.Time   `json:"last_run"`
	LastRunStatus string      `json:"last_run_status"`
	Status        int         `json:"status"`
}

type EriusTagInfo struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type EriusScenario struct {
	ID        uuid.UUID            `json:"id"`
	VersionID uuid.UUID            `json:"version_id"`
	Status    int                  `json:"status"`
	HasDraft  bool                 `json:"hasDraft,omitempty"`
	Name      string               `json:"name"`
	Input     []EriusFunctionValue `json:"input"`
	Output    []EriusFunctionValue `json:"output"`
	Pipeline  struct {
		Entrypoint string               `json:"entrypoint"`
		Blocks     map[string]EriusFunc `json:"blocks"`
	} `json:"pipeline"`
}

type EriusFunctionList struct {
	Functions []script.FunctionModel `json:"funcs"`
	Shapes    []script.ShapeModel    `json:"shapes"`
}

type EriusFunc struct {
	X         int                  `json:"x,omitempty"`
	Y         int                  `json:"y,omitempty"`
	BlockType string               `json:"block_type"`
	Title     string               `json:"title"`
	Input     []EriusFunctionValue `json:"input"`
	Output    []EriusFunctionValue `json:"output,omitempty"`
	OnTrue    string               `json:"on_true,omitempty"`
	OnFalse   string               `json:"on_false,omitempty"`
	Next      string               `json:"next,omitempty"`
}

type EriusFunctionValue struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Global string `json:"global,omitempty"`
}

type UsageResponse struct {
	Name string `json:"name"`
	UsedBy []uuid.UUID `json:"used_by"`
}

type Shapes struct {
	Shapes []script.ShapeModel `json:"shapes"`
}
