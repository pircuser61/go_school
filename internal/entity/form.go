package entity

import "github.com/iancoleman/orderedmap"

type UserExecutionType string
type BlockType string

const (
	UserExecution       UserExecutionType = "user"
	FromSchemaExecution UserExecutionType = "from_schema"
	GroupExecution      UserExecutionType = "group"

	ExecutionBlockType   BlockType = "execution"
	ApprovementBlockType BlockType = "approver"
)

type UsersWithFormAccess struct {
	GroupId       *string           `json:"executors_group_id"`
	ExecutionType UserExecutionType `json:"execution_type"`
	Executor      string            `json:"executor"`
	BlockType     BlockType         `json:"block_type"`
}

type DescriptionForm struct {
	Name        string
	Description orderedmap.OrderedMap
}
