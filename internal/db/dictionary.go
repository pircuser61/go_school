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

	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}

	defer conn.Release()

	rows, err := conn.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	items := make([]entity.ApproveActionName, 0)

	for rows.Next() {
		item := entity.ApproveActionName{}

		if err := rows.Scan(&item.Id, &item.Title); err != nil {
			return nil, err
		}

		items = append(items, item)
	}

	defer rows.Close()

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

	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}

	defer conn.Release()

	rows, err := conn.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	items := make([]entity.ApproveStatus, 0)

	for rows.Next() {
		item := entity.ApproveStatus{}

		if err := rows.Scan(&item.Id, &item.Title); err != nil {
			return nil, err
		}

		items = append(items, item)
	}

	defer rows.Close()

	return items, nil
}
