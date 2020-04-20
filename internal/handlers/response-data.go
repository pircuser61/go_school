package handlers

import (
	"github.com/google/uuid"
	"time"
)

type PipelinesList struct {
	Pipelines []PipelineInfo `json:"pipelines"`
	Drafts    []PipelineInfo `json:"drafts"`
	OnApprove []PipelineInfo `json:"on_approve"`
	Tags      []TagInfo      `json:"tags"`
}
type PipelineInfo struct {
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
	Status        string      `json:"status"`
}

type TagInfo struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type Pipeline struct {
	ID        uuid.UUID       `json:"id"`
	VersionID uuid.UUID       `json:"version_id"`
	Status    int             `json:"status"`
	HasDraft  bool            `json:"hasDraft"`
	Name      string          `json:"name"`
	Input     []FunctionValue `json:"input"`
	Output    []FunctionValue `json:"output"`
	Pipeline  struct {
		Entrypoint string `json:"entrypoint"`
		Blocks     map[string]struct {
			X       string          `json:"x"`
			Y       string          `json:"y"`
			Kind    string          `json:"kind"`
			Input   []FunctionValue `json:"input"`
			Output  []FunctionValue `json:"output,omitempty"`
			OnTrue  string          `json:"on_true,omitempty"`
			OnFalse string          `json:"on_false,omitempty"`
			Next string `json:"next,omitempty"`
		} `json:"blocks"`
	} `json:"pipeline"`
}

type FunctionValue struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Global string `json:"global"`
}
