package entity

import "encoding/json"

type CreateTaskEvent struct {
	WorkID    string
	Author    string
	EventType string
	Params    json.RawMessage
}
