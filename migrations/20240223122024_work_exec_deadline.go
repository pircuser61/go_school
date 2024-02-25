package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/pressly/goose/v3"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sla"
)

func init() {
	goose.AddMigration(upWorkExecDeadline, downWorkExecDeadline)
}

func upWorkExecDeadline(tx *sql.Tx) error {
	err := updateWorksAddColumn(tx)
	if err != nil {
		log.Printf("couldn't add column: %v", err)
		return err
	}

	ww, err := getWorksToUpdate(tx)
	if err != nil {
		log.Printf("couldn't get works: %v", err)
		return err
	}

	srv := sla.NewSLAService(nil)

	err = computeWorksDeadlines(srv, ww)
	if err != nil {
		log.Printf("couldn't compute deadlines: %v", err)
		return err
	}

	err = createAndFillTempExecDeadlineTable(tx, ww)
	if err != nil {
		log.Printf("couldn't fill temp: %v", err)
		return err
	}

	err = updateWorksDeadlines(tx)
	if err != nil {
		log.Printf("couldn't update works: %v", err)
		return err
	}

	err = dropTempExecDeadlineTable(tx)
	if err != nil {
		log.Printf("couldn't drop temp: %v", err)
		return err
	}

	return nil
}

func updateWorksAddColumn(tx *sql.Tx) error {
	_, alterErr := tx.Exec(`ALTER TABLE works ADD COLUMN exec_deadline timestamp with time zone`)
	if alterErr != nil {
		return alterErr
	}

	return nil
}

type workToAddDeadline struct {
	workNumber   string
	workID       string
	startedAt    time.Time
	slaWorkType  string
	slaVal       int
	currDeadline time.Time
}

func getWorksToUpdate(tx *sql.Tx) ([]*workToAddDeadline, error) {
	const q = `
WITH blocks AS (
    SELECT work_id, min(content -> 'State' -> step_name ->> 'deadline') as time
    FROM variable_storage vs
    WHERE step_type = 'execution'
      AND status = 'running'
    GROUP BY work_id
)
SELECT work_number
     , w.id
     , w.started_at
     , coalesce(vsla.work_type, '8/5')
     , coalesce(vsla.sla, 40)
     , coalesce(b.time, '')
FROM pipeliner.public.works w
         LEFT JOIN pipeliner.public.version_sla vsla ON w.version_sla_id = vsla.id
         LEFT JOIN blocks b ON b.work_id = w.id`

	rows, err := tx.Query(q)
	if err != nil {
		return nil, err
	}

	works := make([]*workToAddDeadline, 0, 10000)

	var deadline string

	loc, _ := time.LoadLocation("Europe/Moscow")

	for rows.Next() {
		w := &workToAddDeadline{}

		scanErr := rows.Scan(&w.workNumber, &w.workID, &w.startedAt, &w.slaWorkType, &w.slaVal, &deadline)
		if scanErr != nil {
			return nil, scanErr
		}

		if deadline != "" {
			parsed, deadErr := time.ParseInLocation(time.RFC3339, deadline, loc)
			if deadErr != nil {
				return nil, deadErr
			}

			w.currDeadline = parsed
		}

		works = append(works, w)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return works, nil
}

func computeWorksDeadlines(srv sla.Service, ww []*workToAddDeadline) error {
	for _, w := range ww {
		if !w.currDeadline.IsZero() {
			continue
		}

		slaInfoPtr, getSLAInfoErr := srv.GetSLAInfoPtr(context.Background(), sla.InfoDTO{
			TaskCompletionIntervals: []entity.TaskCompletionInterval{
				{
					StartedAt:  w.startedAt,
					FinishedAt: w.startedAt.Add(time.Hour * 24 * 100),
				},
			},
			WorkType: sla.WorkHourType(w.slaWorkType),
		})
		if getSLAInfoErr != nil {
			return getSLAInfoErr
		}

		w.currDeadline = srv.ComputeMaxDate(
			w.startedAt,
			float32(w.slaVal),
			slaInfoPtr)
	}

	return nil
}

func insertTempExecDeadline(tx *sql.Tx, ww []*workToAddDeadline) error {
	valueStrings := make([]string, 0, len(ww))
	valueArgs := make([]interface{}, 0, len(ww))
	for _, w := range ww {
		valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d)", len(valueArgs)+1, len(valueArgs)+2))

		valueArgs = append(valueArgs, w.workID)
		valueArgs = append(valueArgs, w.currDeadline)
	}

	q := fmt.Sprintf(`INSERT INTO temp_exec_deadlines(work_id, deadline) VALUES %s`, strings.Join(valueStrings, ","))
	_, insertErr := tx.Exec(q, valueArgs...)
	if insertErr != nil {
		return fmt.Errorf("couldn't insert data: %w", insertErr)
	}

	return nil
}

func createAndFillTempExecDeadlineTable(tx *sql.Tx, ww []*workToAddDeadline) error {
	_, crErr := tx.Exec(`CREATE TABLE temp_exec_deadlines (
    work_id uuid,
    deadline timestamp with time zone
)`)
	if crErr != nil {
		return crErr
	}

	batchSize := 1000

	for i := 0; i < (len(ww)/batchSize)+1; i++ {
		start := i * batchSize
		end := (i + 1) * batchSize
		if end > len(ww) {
			end = len(ww)
		}
		part := ww[start:end]

		insertErr := insertTempExecDeadline(tx, part)
		if insertErr != nil {
			return insertErr
		}
	}

	return nil
}

func updateWorksDeadlines(tx *sql.Tx) error {
	_, updErr := tx.Exec(`
UPDATE works 
SET exec_deadline = temp_exec_deadlines.deadline 
FROM temp_exec_deadlines
WHERE works.id = temp_exec_deadlines.work_id`)
	if updErr != nil {
		return updErr
	}

	return nil
}

func dropTempExecDeadlineTable(tx *sql.Tx) error {
	_, dropErr := tx.Exec(`DROP TABLE temp_exec_deadlines`)
	if dropErr != nil {
		return dropErr
	}

	return nil
}

func downWorkExecDeadline(tx *sql.Tx) error {
	_, err := tx.Exec(`ALTER TABLE works DROP COLUMN exec_deadline`)
	return err
}
