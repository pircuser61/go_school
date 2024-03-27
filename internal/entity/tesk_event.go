package entity

import "encoding/json"

type CreateTaskEvent struct {
	WorkID    string
	Author    string
	EventType string
	Params    json.RawMessage
}

type CreateEventToSend struct {
	WorkID  string
	Message json.RawMessage
}
