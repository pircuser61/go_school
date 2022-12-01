package db

import (
	"context"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

//nolint:dupl //its not duplicate
func (db *PGCon) GetApproveActionNames(ctx context.Context) ([]entity.ApproveActionName, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_approve_action_names")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	const query = `
		SELECT  id, title
			FROM dict_approve_action_names 
				WHERE deleted_at IS NULL
			ORDER BY created_at DESC`

	rows, err := db.Connection.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]entity.ApproveActionName, 0)

	for rows.Next() {
		item := entity.ApproveActionName{}

		if scanErr := rows.Scan(&item.Id, &item.Title); scanErr != nil {
			return nil, scanErr
		}

		items = append(items, item)
	}

	return items, nil
}

//nolint:dupl //its not duplicate
func (db *PGCon) GetApproveStatuses(ctx context.Context) ([]entity.ApproveStatus, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_approve_statuses")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	const query = `
		SELECT  id, title
			FROM dict_approve_statuses 
				WHERE deleted_at IS NULL
			ORDER BY created_at DESC`

	rows, err := db.Connection.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]entity.ApproveStatus, 0)

	for rows.Next() {
		item := entity.ApproveStatus{}

		if scanErr := rows.Scan(&item.Id, &item.Title); scanErr != nil {
			return nil, scanErr
		}

		items = append(items, item)
	}

	return items, nil
}
