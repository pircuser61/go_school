package storage

import (
	"context"

	"github.com/google/uuid"

	"github.com/pircuser61/go_school/internal/models"
)

type MateralStore interface {
	MaterialCreate(context.Context, models.Material) (uuid.UUID, error)
	MaterialGet(context.Context, uuid.UUID) (models.Material, error)
	MaterialUpdate(context.Context, models.Material) error
	MaterialDelete(context.Context, uuid.UUID) error
	Materials(context.Context, models.MaterialListFilter) ([]models.MaterialListItem, error)
	Close()
}
