package entity

import "github.com/iancoleman/orderedmap"

type (
	UserExecutionType string
	BlockType         string
)

type UsersWithFormAccess struct {
	GroupID       *string           `json:"executors_group_id"`
	ExecutionType UserExecutionType `json:"execution_type"`
	Executor      string            `json:"executor"`
	BlockType     BlockType         `json:"block_type"`
}

type DescriptionForm struct {
	Name        string
	Description orderedmap.OrderedMap
}
