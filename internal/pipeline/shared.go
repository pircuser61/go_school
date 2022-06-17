package pipeline

import "github.com/google/uuid"

type UpdateData struct {
	Id   uuid.UUID
	Data interface{}
}
