package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"gitlab.services.mts.ru/erius/pipeliner/internal/dbconn"
	"gitlab.services.mts.ru/erius/pipeliner/internal/entity"
	"go.opencensus.io/trace"
)

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

var (
	errCantFindPipelineVersion = errors.New("can't find pipeline version")
)

type PipelineStorageModelDepricated struct {
	ID        uuid.UUID
	Name      string
	CreatedAt time.Time
	DeletedAt time.Time
	Author    string
	Pipeline  string
}

func parseRowsVersionList(ctx context.Context, rows pgx.Rows) ([]entity.EriusScenarioInfo, error) {
	_, span := trace.StartSpan(ctx, "parse_row_version_list")
	defer span.End()

	defer rows.Close()

	versionInfoList := make([]entity.EriusScenarioInfo, 0)

	for rows.Next() {
		e := entity.EriusScenarioInfo{}

		var approver sql.NullString

		err := rows.Scan(&e.VersionID, &e.Status, &e.ID, &e.CreatedAt, &e.Author, &approver, &e.Name)
		if err != nil {
			return nil, err
		}

		e.Approver = approver.String
		versionInfoList = append(versionInfoList, e)
	}

	return versionInfoList, nil
}

func GetApprovedVersions(c context.Context, pc *dbconn.PGConnection) ([]entity.EriusScenarioInfo, error) {
	c, span := trace.StartSpan(c, "pg_list_approved_versions")
	defer span.End()

	return getVersionsByStatus(c, pc, StatusApproved)
}

func getVersionsByStatus(c context.Context, pc *dbconn.PGConnection, status int) ([]entity.EriusScenarioInfo, error) {
	c, span := trace.StartSpan(c, "pg_get_versions_by_status")
	defer span.End()

	q := `SELECT 
	pv.id, pv.status, pv.pipeline_id, pv.created_at, pv.author, pv.approver, pp.name
from pipeliner.versions pv
join pipeliner.pipelines pp on pv.pipeline_id = pp.id
where 
	pv.status = $1
and pp.deleted_at is NULL
order by created_at `

	rows, err := pc.Pool.Query(c, q, status)
	if err != nil {
		return nil, err
	}

	return parseRowsVersionList(c, rows)
}

func GetDraftVersions(c context.Context, pc *dbconn.PGConnection, author string) ([]entity.EriusScenarioInfo, error) {
	c, span := trace.StartSpan(c, "pg_list_draft_versions")
	defer span.End()

	return getVersionsByStatusAndAuthor(c, pc, StatusDraft, author)
}

func GetOnApproveVersions(c context.Context, pc *dbconn.PGConnection) ([]entity.EriusScenarioInfo, error) {
	c, span := trace.StartSpan(c, "pg_list_on_approve_versions")
	defer span.End()

	return getVersionsByStatus(c, pc, StatusOnApprove)
}

func GetWorkedVersions(c context.Context, pc *dbconn.PGConnection) ([]entity.EriusScenario, error) {
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

	rows, err := pc.Pool.Query(c, q, StatusDeleted)
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

func getVersionsByStatusAndAuthor(c context.Context, pc *dbconn.PGConnection,
	status int, author string) ([]entity.EriusScenarioInfo, error) {
	c, span := trace.StartSpan(c, "pg_get_version_by_status_and_author")
	defer span.End()

	q := `SELECT 
	pv.id, pv.status, pv.pipeline_id, pv.created_at, pv.author, pv.approver, pp.name
from pipeliner.versions pv
join pipeliner.pipelines pp on pv.pipeline_id = pp.id
where 
	pv.status = $1
and pv.author = $2
and pp.deleted_at is NULL
order by created_at `

	rows, err := pc.Pool.Query(c, q, status, author)
	if err != nil {
		return nil, err
	}

	return parseRowsVersionList(c, rows)
}

func SwitchApproved(c context.Context, pc *dbconn.PGConnection, pipelineID, versionID uuid.UUID, author string) error {
	c, span := trace.StartSpan(c, "pg_switch_approved")
	defer span.End()

	date := time.Now()

	conn, err := pc.Pool.Acquire(c)
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

func VersionEditable(c context.Context, pc *dbconn.PGConnection, versionID uuid.UUID) (bool, error) {
	c, span := trace.StartSpan(c, "pg_version_editable")
	defer span.End()

	q := `select count(id) from pipeliner.versions where id =$1 and status = $2 or status = $3`

	rows, err := pc.Pool.Query(c, q, versionID, StatusApproved, StatusRejected)
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

func CreatePipeline(c context.Context, pc *dbconn.PGConnection,
	p *entity.EriusScenario, author string, pipelineData []byte) error {
	_, span := trace.StartSpan(c, "pg_create_pipeline")
	defer span.End()

	conn, err := pc.Pool.Acquire(c)
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

	return CreateVersion(c, pc, p, author, pipelineData)
}

func CreateVersion(c context.Context, pc *dbconn.PGConnection,
	p *entity.EriusScenario, author string, pipelineData []byte) error {
	_, span := trace.StartSpan(c, "pg_create_version")
	defer span.End()

	qNewVersion := `INSERT INTO pipeliner.versions(
	id, status, pipeline_id, created_at, content, author)
	VALUES ($1, $2, $3, $4, $5, $6);`

	createdAt := time.Now()

	_, err := pc.Pool.Exec(c, qNewVersion, p.VersionID, StatusDraft, p.ID, createdAt, pipelineData, author)
	if err != nil {
		return err
	}

	return nil
}

func DeleteVersion(c context.Context, pc *dbconn.PGConnection, versionID uuid.UUID) error {
	_, span := trace.StartSpan(c, "pg_delete_version")
	defer span.End()

	q := `UPDATE pipeliner.versions SET deleted_at=$1, status=$2 WHERE id = $3`
	t := time.Now()

	_, err := pc.Pool.Exec(c, q, t, StatusDeleted, versionID)
	if err != nil {
		return err
	}

	qPid := `SELECT pipeline_id from pipeliner.versions  where id = $1`
	row := pc.Pool.QueryRow(c, qPid, versionID)
	var pid uuid.UUID
	err = row.Scan(&pid)
	if err != nil {
		fmt.Println(err)
		return err
	}
	return DeletePipeline(c, pc, pid)

}

func DeletePipeline(c context.Context, pc *dbconn.PGConnection, id uuid.UUID) error {
	c, span := trace.StartSpan(c, "pg_delete_pipeline")
	defer span.End()
	t := time.Now()
	qName := `SELECT name from pipeliner.pipelines WHERE id = $1`
	row := pc.Pool.QueryRow(c, qName, id)
	var n string
	err := row.Scan(&n)
	if err != nil {
		fmt.Println(err)
		return err
	}
	n = n + "_deleted_at_" + t.String()
	fmt.Println(n)
	q := `UPDATE pipeliner.pipelines SET deleted_at=$1, name=$2  WHERE id = $3`

	_, err = pc.Pool.Exec(c, q, t, n, id)
	if err != nil {
		fmt.Println(err)
		return err
	}

	return nil
}

func GetPipeline(c context.Context, pc *dbconn.PGConnection, id uuid.UUID) (*entity.EriusScenario, error) {
	c, span := trace.StartSpan(c, "pg_get_pipeline")
	defer span.End()

	conn, err := pc.Pool.Acquire(c)
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

	return nil, nil
}

func GetPipelineVersion(c context.Context, pc *dbconn.PGConnection, id uuid.UUID) (*entity.EriusScenario, error) {
	c, span := trace.StartSpan(c, "pg_get_pipeline_version")
	defer span.End()

	conn, err := pc.Pool.Acquire(c)
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

func UpdateDraft(c context.Context, pc *dbconn.PGConnection,
	p *entity.EriusScenario, pipelineData []byte) error {
	c, span := trace.StartSpan(c, "pg_update_draft")
	defer span.End()

	q := `UPDATE pipeliner.versions SET
	 status = $1, content =$2 WHERE id = $3;`

	_, err := pc.Pool.Exec(c, q, p.Status, pipelineData, p.VersionID)
	if err != nil {
		return err
	}

	return nil
}

func WriteContext(c context.Context, pc *dbconn.PGConnection, workID uuid.UUID, stage string, data []byte) error {
	c, span := trace.StartSpan(c, "pg_write_context")
	defer span.End()

	conn, err := pc.Pool.Acquire(c)
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

func WriteTask(c context.Context, pc *dbconn.PGConnection,
	workID, versionID uuid.UUID, author string) error {
	c, span := trace.StartSpan(c, "pg_write_task")
	defer span.End()

	conn, err := pc.Pool.Acquire(c)
	if err != nil {
		return err
	}

	defer conn.Release()

	timestamp := time.Now()
	q := `
INSERT INTO pipeliner.works(
	id, version_id, started_at, status, author)
	VALUES ($1, $2, $3, $4, $5);
`

	_, err = conn.Exec(c, q, workID, versionID, timestamp, RunStatusRunned, author)
	if err != nil {
		return err
	}

	return nil
}

func ChangeWorkStatus(c context.Context, pc *dbconn.PGConnection,
	workID uuid.UUID, status int) error {
	c, span := trace.StartSpan(c, "pg_change_work_status")
	defer span.End()

	conn, err := pc.Pool.Acquire(c)
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

func GetExecutableScenarios(c context.Context, pc *dbconn.PGConnection) ([]entity.EriusScenario, error) {
	c, span := trace.StartSpan(c, "pg_all_not_deleted_versions")
	defer span.End()

	q := `
	SELECT pv.id, pp.name, pv.status, pv.pipeline_id, pv.content
from pipeliner.versions pv
join pipeliner.pipelines pp on pv.pipeline_id = pp.id
where 
	pv.status = $1
and pp.deleted_at is NULL
order by pv.created_at `

	rows, err := pc.Pool.Query(c, q, StatusApproved)
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

func GetExecutableByName(c context.Context, pc *dbconn.PGConnection, name string) (*entity.EriusScenario, error) {
	c, span := trace.StartSpan(c, "pg_get_pipeline")
	defer span.End()

	conn, err := pc.Pool.Acquire(c)
	if err != nil {
		return nil, err
	}

	defer conn.Release()

	p := entity.EriusScenario{}
	q := `
SELECT pv.id, pv.status, pv.pipeline_id, pv.content
	FROM pipeliner.versions pv
JOIN pipeliner.pipeline_history pph on pph.version_id = pv.id
JOIN pipeliner.pipelines p on p.id = p.pipeline_id
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
