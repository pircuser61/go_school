package db

import (
	c "context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v4"

	"go.opencensus.io/trace"

	e "gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

func (db *PGCon) CreateTaskStepInputs(ctx c.Context, in *e.CreateTaskStepInputs) (err error) {
	ctx, span := trace.StartSpan(ctx, "create_task_step_inputs")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	const q = `
		INSERT INTO task_steps_inputs (
			work_id, 
			event_id, 
			step_name, 
			author, 
			content, 
			created_at
		)
		VALUES (
			$1, 
			$2, 
			$3, 
			$4, 
			$5, 
		    now()
		)`

	_, err = db.Connection.Exec(ctx, q, in.WorkID, in.EventID, in.StepName, in.Author, in.Inputs)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGCon) GetStepDataFromVersion(ctx c.Context, workNumber, stepName string) (*e.EriusFunc, error) {
	ctx, span := trace.StartSpan(ctx, "get_step_data_from_version")
	defer span.End()

	const q = `
		SELECT content->'pipeline'->'blocks'->$1 
		FROM versions
    	JOIN works w ON versions.id = w.version_id
		WHERE w.work_number = $2 AND w.child_id IS NULL`

	var step *e.EriusFunc

	if err := db.Connection.QueryRow(ctx, q, stepName, workNumber).Scan(&step); err != nil {
		return nil, err
	}

	if step == nil {
		return nil, errors.New("couldn't find step data")
	}

	inputs, err := db.GetStepInputs(ctx, stepName, workNumber, time.Time{})
	if err != nil {
		return nil, err
	}

	step.Params, err = trySetNewParams(step.Params, inputs)
	if err != nil {
		return nil, err
	}

	return step, nil
}

func trySetNewParams(stepParams json.RawMessage, inputs e.BlockInputs) (json.RawMessage, error) {
	if inputs == nil {
		return stepParams, nil
	}

	if len(stepParams) == 0 {
		return stepParams, nil
	}

	inputsFromVersion := make(map[string]interface{}, 0)

	err := json.Unmarshal(stepParams, &inputsFromVersion)
	if err != nil {
		return stepParams, err
	}

	for i := range inputs {
		for inputName := range inputsFromVersion {
			if inputs[i].Name == inputName {
				inputsFromVersion[inputName] = inputs[i].Value
			}
		}
	}

	stepParams, err = json.Marshal(&inputsFromVersion)
	if err != nil {
		return stepParams, err
	}

	return stepParams, nil
}

const getInputsQuery = `
	SELECT content
	FROM task_steps_inputs ts
	WHERE ts.work_id = (SELECT id FROM works WHERE work_number = $1 AND child_id IS NULL LIMIT 1) AND 
		ts.step_name = $2`

const getInputsQueryOrder = " ORDER BY ts.created_at DESC LIMIT 1"

func (db *PGCon) GetStepInputs(ctx c.Context, stepName, workNumber string, createdAt time.Time) (e.BlockInputs, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_step_inputs")
	defer span.End()

	res := make(e.BlockInputs, 0)
	inputs := make(map[string]interface{}, 0)

	queryParams := []interface{}{
		workNumber,
		stepName,
	}

	query := getInputsQuery

	if !createdAt.IsZero() {
		query = fmt.Sprintf("%s %s", query, `AND ts.created_at < $3`)

		queryParams = append(queryParams, createdAt)
	}

	query += getInputsQueryOrder

	err := db.Connection.QueryRow(ctx, query, queryParams...).Scan(&inputs)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return res, nil
		}
	}

	if len(inputs) == 0 {
		const getInputsByVersionQuery = `
			SELECT content -> 'pipeline' -> 'blocks' -> $1 -> 'params'
			FROM versions
			JOIN works w ON versions.id = w.version_id
			WHERE w.work_number = $2 AND w.child_id IS NULL`

		err = db.Connection.QueryRow(ctx, getInputsByVersionQuery, stepName, workNumber).Scan(&inputs)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return res, nil
			}

			return nil, err
		}
	}

	for i := range inputs {
		res = append(res, e.BlockInputValue{
			Name:  i,
			Value: inputs[i],
		})
	}

	return res, nil
}

func (db *PGCon) GetEditedStepInputs(ctx c.Context, stepName, workNumber string, updatedAt *time.Time) (e.BlockInputs, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_edited_step_inputs")
	defer span.End()

	res := make(e.BlockInputs, 0)
	inputs := make(map[string]interface{}, 0)

	queryParams := []interface{}{
		workNumber,
		stepName,
	}

	query := getInputsQuery

	if updatedAt != nil && !updatedAt.IsZero() {
		query = fmt.Sprintf("%s %s", query, `AND ts.created_at < $3`)
		queryParams = append(queryParams, updatedAt)
	}

	//nolint:all // ok
	query += getInputsQueryOrder

	err := db.Connection.QueryRow(ctx, query, queryParams...).Scan(&inputs)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return res, nil
		}
	}

	for i := range inputs {
		res = append(res, e.BlockInputValue{
			Name:  i,
			Value: inputs[i],
		})
	}

	return res, nil
}
