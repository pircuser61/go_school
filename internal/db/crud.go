package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"go.opencensus.io/trace"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"

	"gitlab.services.mts.ru/erius/pipeliner/internal/configs"
	"gitlab.services.mts.ru/erius/pipeliner/internal/ctx"
	"gitlab.services.mts.ru/erius/pipeliner/internal/entity"
	"gitlab.services.mts.ru/erius/pipeliner/internal/store"
)

type PGConnection struct {
	Pool *pgxpool.Pool
}

func ConnectPostgres(db *configs.Database) (PGConnection, error) {
	maxConnections := strconv.Itoa(db.MaxConnections)
	connString := "postgres://" + db.User + ":" + db.Pass + "@" + db.Host + ":" + db.Port + "/" + db.DBName +
		"?sslmode=disable&pool_max_conns=" + maxConnections

	conn, err := pgxpool.Connect(ctx.Context(db.Timeout), connString)
	if err != nil {
		return PGConnection{}, err
	}

	pgc := PGConnection{Pool: conn}
	return pgc, nil
}

const (
	StatusDraft     int = 1
	StatusApproved  int = 2
	StatusDeleted   int = 3
	StatusRejected  int = 4
	StatusOnApprove int = 5

	RunStatusRunned   int = 1
	RunStatusFinished int = 2
	RunStatusError    int = 3
)

var errCantFindPipelineVersion = errors.New("can't find pipeline version")

func parseRowsVersionList(c context.Context, rows pgx.Rows) ([]entity.EriusScenarioInfo, error) {
	_, span := trace.StartSpan(c, "parse_row_version_list")
	defer span.End()

	defer rows.Close()

	versionInfoList := make([]entity.EriusScenarioInfo, 0)

	for rows.Next() {
		e := entity.EriusScenarioInfo{}

		var approver sql.NullString

		err := rows.Scan(&e.VersionID, &e.Status, &e.ID, &e.CreatedAt, &e.Author, &approver, &e.Name,
			&e.LastRun, &e.LastRunStatus)
		if err != nil {
			return nil, err
		}

		e.Approver = approver.String

		versionInfoList = append(versionInfoList, e)
	}

	return versionInfoList, nil
}

func (db *PGConnection) GetApprovedVersions(c context.Context) ([]entity.EriusScenarioInfo, error) {
	c, span := trace.StartSpan(c, "pg_list_approved_versions")
	defer span.End()

	vMap := make(map[uuid.UUID]entity.EriusScenarioInfo)

	versions, err := db.GetVersionsByStatus(c, StatusApproved)
	if err != nil {
		return nil, err
	}
	for i := range versions {
		version := versions[i]
		if finV, ok := vMap[version.ID]; ok {
			t, err := db.findApproveDate(c, version.VersionID)
			if err != nil {
				return nil, err
			}

			if finV.ApprovedAt.After(t) {
				continue
			}
		}

		vMap[version.ID] = version
	}

	final := make([]entity.EriusScenarioInfo, len(vMap))
	n := 0
	for i := range vMap {
		v := vMap[i]
		final[n] = v
		n++
	}

	return final, nil
}

func (db *PGConnection) findApproveDate(c context.Context, id uuid.UUID) (time.Time, error) {
	c, span := trace.StartSpan(c, "pg_find_approve_time")
	defer span.End()

	q := `SELECT date FROM pipeliner.pipeline_history where version_id = $1 order by date limit 1`

	rows, err := db.Pool.Query(c, q, id)
	if err != nil {
		return time.Time{}, err
	}

	defer rows.Close()

	if rows.Next() {
		date := time.Time{}

		err := rows.Scan(&date)
		if err != nil {
			return time.Time{}, err
		}
	}

	return time.Time{}, nil
}

func (db *PGConnection) GetVersionsByStatus(c context.Context, status int) ([]entity.EriusScenarioInfo, error) {
	c, span := trace.StartSpan(c, "pg_get_versions_by_status")
	defer span.End()

	q := `SELECT 
	pv.id, pv.status, pv.pipeline_id, pv.created_at, pv.author, pv.approver, pp.name, pw.started_at, pws.name
from pipeliner.versions pv
join pipeliner.pipelines pp on pv.pipeline_id = pp.id
left outer join  pipeliner.works pw on pw.id = pv.last_run_id
left outer join  pipeliner.work_status pws on pws.id = pw.status
where 
	pv.status = $1
and pp.deleted_at is NULL
order by created_at `

	rows, err := db.Pool.Query(c, q, status)
	if err != nil {
		return nil, err
	}

	return parseRowsVersionList(c, rows)
}

func (db *PGConnection) GetDraftVersionsAuth(c context.Context) ([]entity.EriusScenarioInfo, error) {
	c, span := trace.StartSpan(c, "pg_list_draft_versions")
	defer span.End()

	return db.GetVersionsByStatus(c, StatusDraft)
}

func (db *PGConnection) GetDraftVersions(c context.Context, author string) ([]entity.EriusScenarioInfo, error) {
	c, span := trace.StartSpan(c, "pg_list_draft_versions")
	defer span.End()

	return db.GetVersionsByStatusAndAuthor(c, StatusDraft, author)
}

func (db *PGConnection) GetOnApproveVersions(c context.Context) ([]entity.EriusScenarioInfo, error) {
	c, span := trace.StartSpan(c, "pg_list_on_approve_versions")
	defer span.End()

	return db.GetVersionsByStatus(c, StatusOnApprove)
}

//nolint:dupl //its unique
func (db *PGConnection) GetWorkedVersions(c context.Context) ([]entity.EriusScenario, error) {
	c, span := trace.StartSpan(c, "pg_all_not_deleted_versions")
	defer span.End()

	q := `
	SELECT pv.id, pp.name, pv.status, pv.pipeline_id, pv.content
from pipeliner.versions pv
join pipeliner.pipelines pp on pv.pipeline_id = pp.id
where 
	pv.status <> $1
and pp.deleted_at is NULL
order by pv.created_at `

	rows, err := db.Pool.Query(c, q, StatusDeleted)
	if err != nil {
		return nil, err
	}

	pipes := make([]entity.EriusScenario, 0)

	for rows.Next() {
		var vID, pID uuid.UUID

		var s int

		var c, name string

		p := entity.EriusScenario{}

		err = rows.Scan(&vID, &name, &s, &pID, &c)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal([]byte(c), &p)
		if err != nil {
			return nil, err
		}

		p.VersionID = vID
		p.ID = pID
		p.Status = s
		p.Name = name
		pipes = append(pipes, p)
	}

	return pipes, nil
}

func (db *PGConnection) GetVersionsByStatusAndAuthor(c context.Context,
	status int, author string) ([]entity.EriusScenarioInfo, error) {
	c, span := trace.StartSpan(c, "pg_get_version_by_status_and_author")
	defer span.End()

	q := `SELECT 
	pv.id, pv.status, pv.pipeline_id, pv.created_at, pv.author, pv.approver, pp.name,  pw.started_at, pws.name
from pipeliner.versions pv
join pipeliner.pipelines pp on pv.pipeline_id = pp.id
left outer join  pipeliner.works pw on pw.id = pv.last_run_id
left outer join  pipeliner.work_status pws on pws.id = pw.status
where 
	pv.status = $1
and pv.author = $2
and pp.deleted_at is NULL
order by created_at `

	rows, err := db.Pool.Query(c, q, status, author)
	if err != nil {
		return nil, err
	}

	return parseRowsVersionList(c, rows)
}

func (db *PGConnection) SwitchApproved(c context.Context, pipelineID, versionID uuid.UUID, author string) error {
	c, span := trace.StartSpan(c, "pg_switch_approved")
	defer span.End()

	date := time.Now()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return err
	}

	defer conn.Release()

	tx, err := conn.Begin(c)
	if err != nil {
		return err
	}

	id := uuid.New()
	qSetApproved := `UPDATE pipeliner.versions SET status=$1, approver = $2 WHERE id = $3`
	qWriteHistory := `INSERT INTO pipeliner.pipeline_history(id, pipeline_id, version_id, date) VALUES ($1, $2, $3, $4)`

	_, err = tx.Exec(c, qSetApproved, StatusApproved, author, versionID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(c, qWriteHistory, id, pipelineID, versionID, date)
	if err != nil {
		return err
	}

	err = tx.Commit(c)
	if err != nil {
		err = tx.Rollback(c)
		if err != nil {
			return err
		}

		return err
	}

	return nil
}

func (db *PGConnection) VersionEditable(c context.Context, versionID uuid.UUID) (bool, error) {
	c, span := trace.StartSpan(c, "pg_version_editable")
	defer span.End()

	q := `select count(id) from pipeliner.versions where id =$1 and status = $2 or status = $3`

	rows, err := db.Pool.Query(c, q, versionID, StatusApproved, StatusRejected)
	if err != nil {
		return false, err
	}

	defer rows.Close()

	for rows.Next() {
		count := 0
		err = rows.Scan(&count)

		if err != nil {
			return false, err
		}

		if count == 0 {
			return true, nil
		}
	}

	return false, nil
}

func (db *PGConnection) CreatePipeline(c context.Context,
	p *entity.EriusScenario, author string, pipelineData []byte) error {
	_, span := trace.StartSpan(c, "pg_create_pipeline")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return err
	}

	defer conn.Release()

	tx, err := conn.Begin(c)
	if err != nil {
		return err
	}

	createdAt := time.Now()

	qNewPipeline := `INSERT INTO pipeliner.pipelines(
	id, name, created_at, author)
	VALUES ($1, $2, $3, $4);`

	_, err = tx.Exec(c, qNewPipeline, p.ID, p.Name, createdAt, author)
	if err != nil {
		return err
	}

	err = tx.Commit(c)
	if err != nil {
		err = tx.Rollback(c)
		if err != nil {
			return err
		}

		return err
	}

	return db.CreateVersion(c, p, author, pipelineData)
}

func (db *PGConnection) CreateVersion(c context.Context,
	p *entity.EriusScenario, author string, pipelineData []byte) error {
	_, span := trace.StartSpan(c, "pg_create_version")
	defer span.End()

	qNewVersion := `INSERT INTO pipeliner.versions(
	id, status, pipeline_id, created_at, content, author)
	VALUES ($1, $2, $3, $4, $5, $6);`

	createdAt := time.Now()

	_, err := db.Pool.Exec(c, qNewVersion, p.VersionID, StatusDraft, p.ID, createdAt, pipelineData, author)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGConnection) DeleteVersion(c context.Context, versionID uuid.UUID) error {
	_, span := trace.StartSpan(c, "pg_delete_version")
	defer span.End()

	q := `UPDATE pipeliner.versions SET deleted_at=$1, status=$2 WHERE id = $3`
	t := time.Now()

	_, err := db.Pool.Exec(c, q, t, StatusDeleted, versionID)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGConnection) DeletePipeline(c context.Context, id uuid.UUID) error {
	c, span := trace.StartSpan(c, "pg_delete_pipeline")
	defer span.End()

	t := time.Now()
	qName := `SELECT name from pipeliner.pipelines WHERE id = $1`
	row := db.Pool.QueryRow(c, qName, id)

	var n string

	err := row.Scan(&n)
	if err != nil {
		return err
	}

	n = n + "_deleted_at_" + t.String()
	q := `UPDATE pipeliner.pipelines SET deleted_at=$1, name=$2  WHERE id = $3`

	_, err = db.Pool.Exec(c, q, t, n, id)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGConnection) GetPipeline(c context.Context, id uuid.UUID) (*entity.EriusScenario, error) {
	c, span := trace.StartSpan(c, "pg_get_pipeline")
	defer span.End()

	pool := db.Pool

	conn, err := pool.Acquire(c)
	if err != nil {
		return nil, err
	}

	defer conn.Release()

	p := entity.EriusScenario{}
	q := `
SELECT pv.id, pv.status, pv.pipeline_id, pv.content
	FROM pipeliner.versions pv
JOIN pipeliner.pipeline_history pph on pph.version_id = pv.id
	WHERE pv.pipeline_id = $1 order by pph.date desc LIMIT 1
`

	rows, err := conn.Query(c, q, id)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	if rows.Next() {
		var vID, pID uuid.UUID

		var s int

		var c string

		err = rows.Scan(&vID, &s, &pID, &c)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal([]byte(c), &p)
		if err != nil {
			return nil, err
		}

		p.VersionID = vID
		p.ID = pID
		p.Status = s

		return &p, nil
	}

	return nil, errCantFindPipelineVersion
}

func (db *PGConnection) GetPipelineVersion(c context.Context, id uuid.UUID) (*entity.EriusScenario, error) {
	c, span := trace.StartSpan(c, "pg_get_pipeline_version")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return nil, err
	}

	defer conn.Release()

	p := entity.EriusScenario{}

	qVersion := `SELECT id, status, pipeline_id, content
	FROM pipeliner.versions 
	WHERE id = $1 LIMIT 1;`

	rows, err := conn.Query(c, qVersion, id)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	if rows.Next() {
		var vID, pID uuid.UUID

		var s int

		var c string

		err := rows.Scan(&vID, &s, &pID, &c)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal([]byte(c), &p)
		if err != nil {
			return nil, err
		}

		p.VersionID = vID
		p.ID = pID
		p.Status = s

		return &p, nil
	}

	return nil, fmt.Errorf("%w: with id: %v", errCantFindPipelineVersion, id)
}

func (db *PGConnection) UpdateDraft(c context.Context,
	p *entity.EriusScenario, pipelineData []byte) error {
	c, span := trace.StartSpan(c, "pg_update_draft")
	defer span.End()

	q := `UPDATE pipeliner.versions SET
	 status = $1, content =$2 WHERE id = $3;`

	_, err := db.Pool.Exec(c, q, p.Status, pipelineData, p.VersionID)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGConnection) WriteContext(c context.Context, workID uuid.UUID, stage string, data []byte) error {
	c, span := trace.StartSpan(c, "pg_write_context")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return err
	}

	defer conn.Release()

	id := uuid.New()
	timestamp := time.Now()
	q := `INSERT INTO pipeliner.variable_storage(
	id, work_id, step_name, content, time)
	VALUES ($1, $2, $3, $4, $5);
`

	_, err = conn.Exec(c, q, id, workID, stage, data, timestamp)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGConnection) WriteTask(c context.Context,
	workID, versionID uuid.UUID, author string) error {
	c, span := trace.StartSpan(c, "pg_write_task")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return err
	}

	defer conn.Release()

	tx, err := conn.Begin(c)
	if err != nil {
		return err
	}

	timestamp := time.Now()
	q := `
INSERT INTO pipeliner.works(
	id, version_id, started_at, status, author)
	VALUES ($1, $2, $3, $4, $5);
`

	_, err = tx.Exec(c, q, workID, versionID, timestamp, RunStatusRunned, author)
	if err != nil {
		return err
	}

	q = `UPDATE pipeliner.versions SET last_run_id=$1 where id = $2`

	_, err = tx.Exec(c, q, workID, versionID)
	if err != nil {
		return err
	}

	err = tx.Commit(c)
	if err != nil {
		err = tx.Rollback(c)
		if err != nil {
			return err
		}

		return err
	}

	return nil
}

func (db *PGConnection) ChangeWorkStatus(c context.Context,
	workID uuid.UUID, status int) error {
	c, span := trace.StartSpan(c, "pg_change_work_status")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return err
	}

	defer conn.Release()

	q := `
UPDATE pipeliner.works SET status = $1 WHERE id = $2
`

	_, err = conn.Exec(c, q, status, workID)
	if err != nil {
		return err
	}

	return nil
}

//nolint:dupl //its unique
func (db *PGConnection) GetExecutableScenarios(c context.Context) ([]entity.EriusScenario, error) {
	c, span := trace.StartSpan(c, "pg_all_not_deleted_versions")
	defer span.End()

	q := `
SELECT pv.id, pp.name, pv.status, pv.pipeline_id, pv.content, ph.date
from pipeliner.versions pv
join pipeliner.pipelines pp on pv.pipeline_id = pp.id
join pipeliner.pipeline_history ph on ph.version_id = pv.id
where 
	pv.status = $1
and pp.deleted_at is NULL
order by pv.created_at `

	rows, err := db.Pool.Query(c, q, StatusApproved)
	if err != nil {
		return nil, err
	}

	pipes := make([]entity.EriusScenario, 0)

	for rows.Next() {
		var vID, pID uuid.UUID

		var s int

		var c, name string

		var d time.Time

		p := entity.EriusScenario{}

		err = rows.Scan(&vID, &name, &s, &pID, &c, &d)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal([]byte(c), &p)
		if err != nil {
			return nil, err
		}

		p.VersionID = vID
		p.ID = pID
		p.Status = s
		p.Name = name
		p.ApproveDate = d
		pipes = append(pipes, p)
	}

	vMap := make(map[uuid.UUID]entity.EriusScenario)

	for i := range pipes {
		version := pipes[i]
		if finV, ok := vMap[version.ID]; ok {
			t, err := db.findApproveDate(c, version.VersionID)
			if err != nil {
				return nil, err
			}

			if finV.ApproveDate.After(t) {
				continue
			}
		}

		vMap[version.ID] = version
	}

	final := make([]entity.EriusScenario, len(vMap))
	n := 0
	for i := range vMap {
		v := vMap[i]
		final[n] = v
		n++
	}

	return final, nil
}

func (db *PGConnection) GetExecutableByName(c context.Context, name string) (*entity.EriusScenario, error) {
	c, span := trace.StartSpan(c, "pg_get_pipeline")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return nil, err
	}

	defer conn.Release()

	p := entity.EriusScenario{}
	q := `
SELECT pv.id, pv.status, pv.pipeline_id, pv.content
	FROM pipeliner.versions pv
JOIN pipeliner.pipeline_history pph on pph.version_id = pv.id
JOIN pipeliner.pipelines p on p.id = pv.pipeline_id
	WHERE p.name = $1 order by pph.date desc LIMIT 1
`

	rows, err := conn.Query(c, q, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if rows.Next() {
		var vID, pID uuid.UUID

		var s int

		var c string

		err = rows.Scan(&vID, &s, &pID, &c)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal([]byte(c), &p)
		if err != nil {
			return nil, err
		}

		p.VersionID = vID
		p.ID = pID
		p.Status = s

		return &p, nil
	}

	return nil, nil
}

func (db *PGConnection) GetPipelineTasks(c context.Context, id uuid.UUID) (*entity.EriusTasks, error) {
	c, span := trace.StartSpan(c, "pg_get_pipeline_tasks")
	defer span.End()

	q := `SELECT w.id, w.started_at, w.status  FROM pipeliner.works w 
		JOIN pipeliner.versions v ON v.id = w.version_id
		JOIN pipeliner.pipelines p ON p.id = v.pipeline_id 
		WHERE p.id = $1
		ORDER BY w.started_at DESC
		LIMIT 100`

	return db.getTasks(c, q, id)
}

func (db *PGConnection) GetVersionTasks(c context.Context, id uuid.UUID) (*entity.EriusTasks, error) {
	c, span := trace.StartSpan(c, "pg_get_pipeline_tasks")
	defer span.End()

	q := `SELECT w.id, w.started_at, w.status  FROM pipeliner.works w 
		JOIN pipeliner.versions v ON v.id = w.version_id
		WHERE v.id = $1
		ORDER BY w.started_at DESC
		LIMIT 100`

	return db.getTasks(c, q, id)
}

func (db *PGConnection) getTasks(c context.Context, q string, id uuid.UUID) (*entity.EriusTasks, error) {
	c, span := trace.StartSpan(c, "pg_get_tasks")
	defer span.End()

	ets := entity.EriusTasks{
		Tasks: make([]entity.EriusTask, 0),
	}

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return nil, err
	}

	defer conn.Release()

	rows, err := conn.Query(c, q, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		et := entity.EriusTask{}

		err := rows.Scan(&et.ID, &et.Time, &et.Status)
		if err != nil {
			return nil, err
		}

		ets.Tasks = append(ets.Tasks, et)
	}

	return &ets, nil
}

func (db *PGConnection) GetTaskLog(c context.Context, id uuid.UUID) (*entity.EriusLog, error) {
	c, span := trace.StartSpan(c, "pg_get_tasks")
	defer span.End()

	el := entity.EriusLog{
		Steps: make([]entity.Step, 0),
	}

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return nil, err
	}

	defer conn.Release()

	q := `
SELECT vs.step_name, vs.time, vs.content 
FROM pipeliner.variable_storage vs 
WHERE work_id = $1
ORDER BY vs.time DESC
`

	rows, err := conn.Query(c, q, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		s := entity.Step{}
		c := ""

		err := rows.Scan(&s.Name, &s.Time, &c)
		if err != nil {
			return nil, err
		}

		storage := store.NewStore()

		err = json.Unmarshal([]byte(c), storage)
		if err != nil {
			return nil, err
		}

		s.Steps = storage.Steps
		s.Errors = storage.Errors
		s.Storage = storage.Values
		el.Steps = append(el.Steps, s)
	}

	return nil, nil
}
