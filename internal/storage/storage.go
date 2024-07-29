package storage

import (
	"context"

	"github.com/pircuser61/go_school/internal/models"
)

type MateralStore interface {
	Materials(context.Context, models.MaterialListFilter) ([]*models.Material, error)
	Close()
}
