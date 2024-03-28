package entity

import (
	"encoding/json"
	"time"
)

type CreateTaskEvent struct {
	WorkID    string
	Author    string
	EventType string
	Params    json.RawMessage
}

type TaskEvent struct {
	ID        string
	WorkID    string
	Author    string
	EventType string
	Params    json.RawMessage
	CreatedAt time.Time
}

type CreateEventToSend struct {
	WorkID  string
	Message json.RawMessage
}
