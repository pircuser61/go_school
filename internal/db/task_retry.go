package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v4"
)

func (db *PGCon) EmptyTasks(ctx context.Context, minLifetime, maxLifetime time.Duration, limit int) ([]*EmptyTask, error) {
	const query = `
	SELECT w.id, w.version_id, w.work_number, w.author, w.run_context
	FROM works w 
	INNER JOIN variable_storage vs ON vs.work_id = w.id
	WHERE 
	w.version_id IS NOT NULL AND 
	now() - vs.time > make_interval(0,0,0,0,0,0,$2) and 
	now() - vs.time < make_interval(0,0,0,0,0,0,$3) and
	w.status = 5
	ORDER BY vs.time asc
	LIMIT $1
	`

	rows, err := db.Connection.Query(ctx, query, limit, minLifetime.Seconds(), maxLifetime.Seconds())
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	return scanEmptyTasks(rows)
}

func scanEmptyTasks(rows pgx.Rows) ([]*EmptyTask, error) {
	emptyTasks := make([]*EmptyTask, 0)

	for rows.Next() {
		var emptyTask EmptyTask
		err := rows.Scan(&emptyTask.WorkID, &emptyTask.VersionID, &emptyTask.WorkNumber, &emptyTask.Author, &emptyTask.RunContext)
		if err != nil {
			return nil, fmt.Errorf("scan empty task, %w", err)
		}

		emptyTasks = append(emptyTasks, &emptyTask)
	}

	return emptyTasks, nil
}
