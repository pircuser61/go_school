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

	RunStatusRunning  int = 1
	RunStatusFinished int = 2
	RunStatusError    int = 3

	qCheckTagIsAttached string = `SELECT COUNT(pipeline_id)
	FROM pipeliner.pipeline_tags    
	WHERE pipeline_id = $1 and tag_id = $2;`
)

var (
	errCantFindPipelineVersion = errors.New("can't find pipeline version")
	errCantFindTag             = errors.New("can't find tag")
)

func parseRowsVersionList(c context.Context, rows pgx.Rows) ([]entity.EriusScenarioInfo, error) {
	_, span := trace.StartSpan(c, "parse_row_version_list")
	defer span.End()

	defer rows.Close()

	versionInfoList := make([]entity.EriusScenarioInfo, 0)

	for rows.Next() {
		e := entity.EriusScenarioInfo{}

		var approver sql.NullString

		err := rows.Scan(&e.VersionID, &e.Status, &e.ID, &e.CreatedAt, &e.Author, &approver, &e.Name,
			&e.LastRun, &e.LastRunStatus, &e.CommentRejected, &e.Comment)
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

		t, err := db.findApproveDate(c, version.VersionID)
		if err != nil {
			return nil, err
		}

		version.ApprovedAt = t

		if finV, ok := vMap[version.ID]; ok {
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

	q := `SELECT date FROM pipeliner.pipeline_history WHERE version_id = $1 ORDER BY date LIMIT 1;`

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

		return date, nil
	}

	return time.Time{}, nil
}

func (db *PGConnection) GetVersionsByStatus(c context.Context, status int) ([]entity.EriusScenarioInfo, error) {
	c, span := trace.StartSpan(c, "pg_get_versions_by_status")
	defer span.End()

	q := `
	SELECT 
		pv.id, pv.status, pv.pipeline_id, pv.created_at, pv.author, pv.approver, pp.name, pw.started_at, pws.name, pv.comment_rejected, pv.comment
	FROM pipeliner.versions pv
	JOIN pipeliner.pipelines pp ON pv.pipeline_id = pp.id
	LEFT OUTER JOIN  pipeliner.works pw ON pw.id = pv.last_run_id
	LEFT OUTER JOIN  pipeliner.work_status pws ON pws.id = pw.status
	WHERE pv.status = $1
	AND pp.deleted_at IS NULL
	ORDER BY created_at;`

	rows, err := db.Pool.Query(c, q, status)
	if err != nil {
		return nil, err
	}

	res, err := parseRowsVersionList(c, rows)
	if err != nil {
		return nil, err
	}

	for i := range res {
		tags, err := db.GetPipelineTag(c, res[i].ID)
		if err != nil {
			return nil, err
		}

		res[i].Tags = tags
	}

	return res, nil
}

func (db *PGConnection) GetDraftVersions(c context.Context) ([]entity.EriusScenarioInfo, error) {
	c, span := trace.StartSpan(c, "pg_list_draft_versions")
	defer span.End()

	return db.GetVersionsByStatus(c, StatusDraft)
}

func (db *PGConnection) GetOnApproveVersions(c context.Context) ([]entity.EriusScenarioInfo, error) {
	c, span := trace.StartSpan(c, "pg_list_on_approve_versions")
	defer span.End()

	return db.GetVersionsByStatus(c, StatusOnApprove)
}

func (db *PGConnection) GetRejectedVersions(c context.Context) ([]entity.EriusScenarioInfo, error) {
	c, span := trace.StartSpan(c, "pg_list_rejected_versions")
	defer span.End()

	return db.GetVersionsByStatus(c, StatusRejected)
}

//nolint:dupl //its unique
func (db *PGConnection) GetWorkedVersions(c context.Context) ([]entity.EriusScenario, error) {
	c, span := trace.StartSpan(c, "pg_all_not_deleted_versions")
	defer span.End()

	q := `
	SELECT pv.id, pp.name, pv.status, pv.pipeline_id, pv.content
	FROM pipeliner.versions pv
	JOIN pipeliner.pipelines pp ON pv.pipeline_id = pp.id
	WHERE 
		pv.status <> $1
	AND pp.deleted_at IS NULL
	ORDER BY pv.created_at;`

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

func (db *PGConnection) GetAllTags(c context.Context) ([]entity.EriusTagInfo, error) {
	c, span := trace.StartSpan(c, "pg_all_tags")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return nil, err
	}

	defer conn.Release()

	q := `SELECT t.id, t.name, t.status, t.color
	FROM pipeliner.tags t
    WHERE 
		t.status <> $1`

	rows, err := conn.Query(c, q, StatusDeleted)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	tags := make([]entity.EriusTagInfo, 0)

	for rows.Next() {
		etag := entity.EriusTagInfo{}

		err = rows.Scan(&etag.ID, &etag.Name, &etag.Status, &etag.Color)
		if err != nil {
			return nil, err
		}

		tags = append(tags, etag)
	}

	return tags, nil
}

func (db *PGConnection) GetPipelineTag(c context.Context, pid uuid.UUID) ([]entity.EriusTagInfo, error) {
	c, span := trace.StartSpan(c, "pg_pipeline_tag")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return nil, err
	}

	defer conn.Release()

	q := `SELECT t.id, t.name, t.status, t.color, t.is_marker
	FROM pipeliner.tags t
	LEFT OUTER JOIN pipeliner.pipeline_tags pt ON pt.tag_id = t.id
    WHERE 
		t.status <> $1 
	AND
		pt.pipeline_id = $2`

	rows, err := conn.Query(c, q, StatusDeleted, pid)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	tags := make([]entity.EriusTagInfo, 0)

	for rows.Next() {
		etag := entity.EriusTagInfo{}

		err = rows.Scan(&etag.ID, &etag.Name, &etag.Status, &etag.Color, &etag.IsMarker)
		if err != nil {
			return nil, err
		}

		tags = append(tags, etag)
	}

	return tags, nil
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

func (db *PGConnection) SwitchRejected(c context.Context, versionID uuid.UUID, comment, author string) error {
	c, span := trace.StartSpan(c, "pg_switch_rejected")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return err
	}

	defer conn.Release()

	qSetRejected := `UPDATE pipeliner.versions SET status=$1, approver = $2, comment_rejected = $3 WHERE id = $4`

	_, err = conn.Exec(c, qSetRejected, StatusRejected, author, comment, versionID)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGConnection) VersionEditable(c context.Context, versionID uuid.UUID) (bool, error) {
	c, span := trace.StartSpan(c, "pg_version_editable")
	defer span.End()

	q := `SELECT COUNT(id) FROM pipeliner.versions WHERE id =$1 AND status = $2`

	rows, err := db.Pool.Query(c, q, versionID, StatusApproved)
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

func (db *PGConnection) PipelineRemovable(c context.Context, id uuid.UUID) (bool, error) {
	c, span := trace.StartSpan(c, "pg_pipeline_removable")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return false, err
	}

	defer conn.Release()

	q := `SELECT COUNT(id) FROM pipeliner.versions WHERE pipeline_id =$1`

	row := conn.QueryRow(c, q, id)

	count := 0

	err = row.Scan(&count)
	if err != nil {
		return false, err
	}

	if count == 1 {
		return true, nil
	}

	return false, nil
}

func (db *PGConnection) DraftPipelineCreatable(c context.Context, id uuid.UUID, author string) (bool, error) {
	c, span := trace.StartSpan(c, "pg_draft_pipeline_creatable")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return false, err
	}

	defer conn.Release()

	q := `SELECT COUNT(id) FROM pipeliner.versions WHERE pipeline_id =$1 AND author = $2 AND (status = $3 OR status = $4 OR status = $5)`

	row := conn.QueryRow(c, q, id, author, StatusDraft, StatusOnApprove, StatusRejected)

	count := 0

	err = row.Scan(&count)
	if err != nil {
		return false, err
	}

	if count == 0 {
		return true, nil
	}

	return false, nil
}

func (db *PGConnection) CreatePipeline(c context.Context,
	p *entity.EriusScenario, author string, pipelineData []byte) error {
	c, span := trace.StartSpan(c, "pg_create_pipeline")
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
	c, span := trace.StartSpan(c, "pg_create_version")
	defer span.End()

	qNewVersion := `INSERT INTO pipeliner.versions(
	id, status, pipeline_id, created_at, content, author, comment)
	VALUES ($1, $2, $3, $4, $5, $6, $7);`

	createdAt := time.Now()

	_, err := db.Pool.Exec(c, qNewVersion, p.VersionID, StatusDraft, p.ID, createdAt, pipelineData, author, p.Comment)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGConnection) PipelineNameCreatable(c context.Context, name string) (bool, error) {
	c, span := trace.StartSpan(c, "pg_check_pipeline_name_for_existence")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return false, err
	}

	defer conn.Release()

	q := `
	SELECT count(name) 
	FROM pipeliner.pipelines
	WHERE name = $1`

	row := conn.QueryRow(c, q, name)

	count := 0

	err = row.Scan(&count)

	if count != 0 {
		return false, err
	}

	return true, nil
}

func (db *PGConnection) CreateTag(c context.Context,
	e *entity.EriusTagInfo, author string) (*entity.EriusTagInfo, error) {
	c, span := trace.StartSpan(c, "pg_create_tag")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return nil, err
	}

	defer conn.Release()

	if e.Name == "" {
		return nil, err
	}

	qCheckTagExisted := `
	SELECT t.id, t.name, t.status, t.color, t.is_marker
	FROM pipeliner.tags t
	WHERE lower(t.name) = lower($1) AND t.status <> $2 and t.is_marker <> $3
	LIMIT 1;`

	rows, err := conn.Query(c, qCheckTagExisted, e.Name, StatusDeleted, e.IsMarker)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	if rows.Next() {
		err = rows.Scan(&e.ID, &e.Name, &e.Status, &e.Color, &e.IsMarker)
		if err != nil {
			return nil, err
		}

		return e, nil
	}

	qNewTag := `INSERT INTO pipeliner.tags(
	id, name, status, author, color, is_marker)
	VALUES ($1, $2, $3, $4, $5, $6)
	RETURNING id, name, status, color, is_marker;`

	row := conn.QueryRow(c, qNewTag, e.ID, e.Name, StatusDraft, author, e.Color, e.IsMarker)

	etag := &entity.EriusTagInfo{}

	err = row.Scan(&etag.ID, &etag.Name, &etag.Status, &etag.Color, &etag.IsMarker)
	if err != nil {
		return nil, err
	}

	return etag, err
}

func (db *PGConnection) DeleteVersion(c context.Context, versionID uuid.UUID) error {
	c, span := trace.StartSpan(c, "pg_delete_version")
	defer span.End()

	q := `UPDATE pipeliner.versions SET deleted_at=$1, status=$2 WHERE id = $3`
	t := time.Now()

	_, err := db.Pool.Exec(c, q, t, StatusDeleted, versionID)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGConnection) DeleteAllVersions(c context.Context, id uuid.UUID) error {
	c, span := trace.StartSpan(c, "pg_delete_version")
	defer span.End()

	q := `UPDATE pipeliner.versions SET deleted_at=$1, status=$2 WHERE pipeline_id = $3`
	t := time.Now()

	_, err := db.Pool.Exec(c, q, t, StatusDeleted, id)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGConnection) DeletePipeline(c context.Context, id uuid.UUID) error {
	c, span := trace.StartSpan(c, "pg_delete_pipeline")
	defer span.End()

	t := time.Now()
	qName := `SELECT name FROM pipeliner.pipelines WHERE id = $1`
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

	return db.DeleteAllVersions(c, id)
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
	SELECT pv.id, pv.status, pv.pipeline_id, pv.content, pv.comment
	FROM pipeliner.versions pv
	JOIN pipeliner.pipeline_history pph ON pph.version_id = pv.id
	WHERE pv.pipeline_id = $1
	ORDER BY pph.date DESC LIMIT 1;
`

	rows, err := conn.Query(c, q, id)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	if rows.Next() {
		var (
			vID, pID uuid.UUID
			s        int
			c        string
			cm       string
		)

		err = rows.Scan(&vID, &s, &pID, &c, &cm)
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
		p.Comment = cm

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

	qVersion := `
	SELECT id, status, pipeline_id, content, comment_rejected, comment
	FROM pipeliner.versions 
	WHERE id = $1 LIMIT 1;`

	rows, err := conn.Query(c, qVersion, id)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	if rows.Next() {
		var (
			vID, pID uuid.UUID
			s        int
			c        string
			cr       string
			cm       string
		)

		err := rows.Scan(&vID, &s, &pID, &c, &cr, &cm)
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
		p.CommentRejected = cr
		p.Comment = cm

		return &p, nil
	}

	return nil, fmt.Errorf("%w: with id: %v", errCantFindPipelineVersion, id)
}

func (db *PGConnection) GetTag(c context.Context, e *entity.EriusTagInfo) (*entity.EriusTagInfo, error) {
	c, span := trace.StartSpan(c, "pg_get_tag")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return nil, err
	}

	defer conn.Release()

	qGetTag := `
	SELECT t.id, t.name, t.status, t.color, t.is_marker
	FROM pipeliner.tags t
	WHERE t.id = $1 AND t.status <> $2
	LIMIT 1;`

	rows, err := conn.Query(c, qGetTag, e.ID, StatusDeleted)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	if rows.Next() {
		err := rows.Scan(&e.ID, &e.Name, &e.Status, &e.Color, &e.IsMarker)
		if err != nil {
			return nil, err
		}

		return e, nil
	}

	return nil, fmt.Errorf("%w: with id: %v", errCantFindTag, e.ID)
}

func (db *PGConnection) EditTag(c context.Context, e *entity.EriusTagInfo) error {
	c, span := trace.StartSpan(c, "pg_edit_tag")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return err
	}

	defer conn.Release()

	qCheckTagIsCreated := `
	SELECT count(id)
	FROM pipeliner.tags 
	WHERE id = $1 AND status = $2`

	row := conn.QueryRow(c, qCheckTagIsCreated, e.ID, StatusDraft)

	count := 0

	err = row.Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		return fmt.Errorf("%w: with id: %v", errCantFindTag, e.ID)
	}

	qEditTag := `UPDATE pipeliner.tags
	SET color = $1
	WHERE id = $2;`

	_, err = conn.Exec(c, qEditTag, e.Color, e.ID)
	if err != nil {
		return err
	}

	return nil
}

//nolint:dupl //its different
func (db *PGConnection) AttachTag(c context.Context, pid uuid.UUID, e *entity.EriusTagInfo) error {
	c, span := trace.StartSpan(c, "pg_edit_tag")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return err
	}

	defer conn.Release()

	row := conn.QueryRow(c, qCheckTagIsAttached, pid, e.ID)

	count := 0

	err = row.Scan(&count)
	if err != nil {
		return err
	}

	if count != 0 {
		return nil
	}

	qAttachTag := `INSERT INTO pipeliner.pipeline_tags (
	pipeline_id, tag_id)
	VALUES ($1, $2);`

	_, err = conn.Exec(c, qAttachTag, pid, e.ID)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGConnection) RemoveTag(c context.Context, id uuid.UUID) error {
	c, span := trace.StartSpan(c, "pg_remove_tag")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return err
	}

	defer conn.Release()

	t := time.Now()

	qName := `SELECT name FROM pipeliner.tags WHERE id = $1`

	row := conn.QueryRow(c, qName, id)

	var n string

	err = row.Scan(&n)
	if err != nil {
		return err
	}

	n = n + "_deleted_at_" + t.String()

	qSetTagDeleted := `UPDATE pipeliner.tags
	SET status=$1, name=$2  
	WHERE id = $3;`

	_, err = conn.Exec(c, qSetTagDeleted, StatusDeleted, n, id)
	if err != nil {
		return err
	}

	qCheckTagAttached := `SELECT COUNT(pipeline_id)
	FROM pipeliner.pipeline_tags
	WHERE tag_id = $1`

	row = conn.QueryRow(c, qCheckTagAttached, id)

	count := 0

	err = row.Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		return nil
	}

	qRemoveAttachedTags := `DELETE FROM pipeliner.pipeline_tags
	WHERE tag_id = $1`

	_, err = conn.Exec(c, qRemoveAttachedTags, id)
	if err != nil {
		return err
	}

	return nil
}

//nolint:dupl //its different
func (db *PGConnection) DetachTag(c context.Context, pid uuid.UUID, e *entity.EriusTagInfo) error {
	c, span := trace.StartSpan(c, "pg_detach_tag")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return err
	}

	defer conn.Release()

	row := conn.QueryRow(c, qCheckTagIsAttached, pid, e.ID)

	count := 0

	err = row.Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		return nil
	}

	qDetachTag := `DELETE FROM pipeliner.pipeline_tags
	WHERE pipeline_id = $1 
	AND tag_id = $2`

	_, err = conn.Exec(c, qDetachTag, pid, e.ID)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGConnection) RemovePipelineTags(c context.Context, id uuid.UUID) error {
	c, span := trace.StartSpan(c, "pg_remove_pipeline_tags")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return err
	}

	defer conn.Release()

	qCheckTagIsAttached := `
	SELECT COUNT(pipeline_id)
	FROM pipeliner.pipeline_tags
	WHERE pipeline_id = $1`

	row := conn.QueryRow(c, qCheckTagIsAttached, id)

	count := 0

	err = row.Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		return nil
	}

	qRemovePipelineTags := `DELETE FROM pipeliner.pipeline_tags
	WHERE pipeline_id = $1`

	_, err = conn.Exec(c, qRemovePipelineTags, id)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGConnection) UpdateDraft(c context.Context,
	p *entity.EriusScenario, pipelineData []byte) error {
	c, span := trace.StartSpan(c, "pg_update_draft")
	defer span.End()

	q := `
	UPDATE pipeliner.versions 
	SET status = $1, content = $2, comment = $3 WHERE id = $4;`

	_, err := db.Pool.Exec(c, q, p.Status, pipelineData, p.Comment, p.VersionID)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGConnection) SaveStepContext(c context.Context, workID uuid.UUID, stage string, data []byte) error {
	c, span := trace.StartSpan(c, "pg_write_context")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return err
	}

	defer conn.Release()

	id := uuid.New()
	timestamp := time.Now()
	q := `
	INSERT INTO pipeliner.variable_storage(
	id, work_id, step_name, content, time)
	VALUES ($1, $2, $3, $4, $5);
`

	_, err = conn.Exec(c, q, id, workID, stage, data, timestamp)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGConnection) CreateTask(c context.Context,
	taskID, versionID uuid.UUID, author string, isDebugMode bool, parameters []byte) (*entity.EriusTask, error) {
	c, span := trace.StartSpan(c, "db_create_task")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return nil, err
	}

	defer conn.Release()

	tx, err := conn.Begin(c)
	if err != nil {
		return nil, err
	}

	startedAt := time.Now()
	q := `
	INSERT INTO pipeliner.works(
	id, version_id, started_at, status, author, debug, parameters)
	VALUES ($1, $2, $3, $4, $5, $6, $7)
	RETURNING id;
`
	row := tx.QueryRow(c, q, taskID, versionID, startedAt, RunStatusRunning, author, isDebugMode, parameters)

	var id uuid.UUID

	err = row.Scan(&id)
	if err != nil {
		err = tx.Rollback(c)
		if err != nil {
			return nil, err
		}

		return nil, err
	}

	q = `UPDATE pipeliner.versions SET last_run_id=$1 WHERE id = $2;`

	_, err = tx.Exec(c, q, taskID, versionID)
	if err != nil {
		err = tx.Rollback(c)
		if err != nil {
			return nil, err
		}

		return nil, err
	}

	err = tx.Commit(c)
	if err != nil {
		err = tx.Rollback(c)
		if err != nil {
			return nil, err
		}

		return nil, err
	}

	return db.GetTask(c, id)
}

func (db *PGConnection) ChangeTaskStatus(c context.Context,
	taskID uuid.UUID, status int) error {
	c, span := trace.StartSpan(c, "pg_change_work_status")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return err
	}

	defer conn.Release()

	q := `UPDATE pipeliner.works SET status = $1 WHERE id = $2;`

	_, err = conn.Exec(c, q, status, taskID)
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
	FROM pipeliner.versions pv
	JOIN pipeliner.pipelines pp ON pv.pipeline_id = pp.id
	JOIN pipeliner.pipeline_history ph ON ph.version_id = pv.id
	WHERE 
		pv.status = $1
		AND pp.deleted_at is NULL
	ORDER BY pv.created_at;`

	rows, err := db.Pool.Query(c, q, StatusApproved)
	if err != nil {
		return nil, err
	}

	pipes := make([]entity.EriusScenario, 0)

	for rows.Next() {
		var (
			vID, pID uuid.UUID
			s        int
			c, name  string
			d        time.Time
		)

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
	WHERE 
		p.name = $1 
		AND p.deleted_at is NULL
	ORDER BY pph.date DESC LIMIT 1;
`

	rows, err := conn.Query(c, q, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if rows.Next() {
		var (
			vID, pID uuid.UUID
			s        int
			c        string
		)

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

func (db *PGConnection) GetPipelineTasks(c context.Context, pipelineID uuid.UUID) (*entity.EriusTasks, error) {
	c, span := trace.StartSpan(c, "pg_get_pipeline_tasks")
	defer span.End()

	q := `SELECT w.id, w.started_at, ws.name, w.debug, w.parameters, w.author, w.version_id
		FROM pipeliner.works w 
		JOIN pipeliner.versions v ON v.id = w.version_id
		JOIN pipeliner.pipelines p ON p.id = v.pipeline_id
		JOIN pipeliner.work_status ws ON w.status = ws.id
		WHERE p.id = $1
		ORDER BY w.started_at DESC
		LIMIT 100;`

	return db.getTasks(c, q, pipelineID)
}

func (db *PGConnection) GetVersionTasks(c context.Context, versionID uuid.UUID) (*entity.EriusTasks, error) {
	c, span := trace.StartSpan(c, "pg_get_pipeline_tasks")
	defer span.End()

	q := `SELECT w.id, w.started_at, ws.name, w.debug, w.parameters,w.author, w.version_id
		FROM pipeliner.works w 
		JOIN pipeliner.versions v ON v.id = w.version_id
		JOIN pipeliner.work_status ws ON w.status = ws.id
		WHERE v.id = $1
		ORDER BY w.started_at DESC
		LIMIT 100;`

	return db.getTasks(c, q, versionID)
}

func (db *PGConnection) GetLastDebugTask(c context.Context, id uuid.UUID, author string) (*entity.EriusTask, error) {
	c, span := trace.StartSpan(c, "pg_get_last_debug_task")
	defer span.End()

	q := `SELECT w.id, w.started_at, ws.name, w.debug, w.parameters, w.author, w.version_id
		FROM pipeliner.works w 
		JOIN pipeliner.versions v ON v.id = w.version_id
		JOIN pipeliner.work_status ws ON w.status = ws.id
		WHERE v.id = $1
		AND w.author = $2
		AND w.debug = true
		ORDER BY w.started_at DESC
		LIMIT 1;`

	et := entity.EriusTask{}

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return nil, err
	}

	defer conn.Release()

	row := conn.QueryRow(c, q, id, author)
	parameters := ""

	err = row.Scan(&et.ID, &et.StartedAt, &et.Status, &et.IsDebugMode, &parameters, &et.Author, &et.VersionID)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal([]byte(parameters), &et.Parameters)
	if err != nil {
		return nil, err
	}

	return &et, nil
}

func (db *PGConnection) GetTask(c context.Context, id uuid.UUID) (*entity.EriusTask, error) {
	c, span := trace.StartSpan(c, "pg_get_task")
	defer span.End()

	q := `SELECT w.id, w.started_at, ws.name, w.debug, w.parameters, w.author, w.version_id
		FROM pipeliner.works w 
		JOIN pipeliner.versions v ON v.id = w.version_id
		JOIN pipeliner.work_status ws ON w.status = ws.id
		WHERE w.id = $1;`

	return db.getTask(c, q, id)
}

func (db *PGConnection) getTask(c context.Context, q string, id uuid.UUID) (*entity.EriusTask, error) {
	c, span := trace.StartSpan(c, "pg_get_task_private")
	defer span.End()

	et := entity.EriusTask{}

	var nullStringParameters sql.NullString

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return nil, err
	}

	defer conn.Release()

	row := conn.QueryRow(c, q, id)

	err = row.Scan(
		&et.ID,
		&et.StartedAt,
		&et.Status,
		&et.IsDebugMode,
		&nullStringParameters,
		&et.Author,
		&et.VersionID)
	if err != nil {
		return nil, err
	}

	if nullStringParameters.Valid {
		err = json.Unmarshal([]byte(nullStringParameters.String), &et.Parameters)
		if err != nil {
			return nil, err
		}
	}

	return &et, nil
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

		var nullStringParameters sql.NullString

		err = rows.Scan(
			&et.ID,
			&et.StartedAt,
			&et.Status,
			&et.IsDebugMode,
			&nullStringParameters,
			&et.Author,
			&et.VersionID)
		if err != nil {
			return nil, err
		}

		if nullStringParameters.Valid {
			err = json.Unmarshal([]byte(nullStringParameters.String), &et.Parameters)
			if err != nil {
				return nil, err
			}
		}

		ets.Tasks = append(ets.Tasks, et)
	}

	return &ets, nil
}

func (db *PGConnection) GetTaskSteps(c context.Context, id uuid.UUID) (entity.TaskSteps, error) {
	c, span := trace.StartSpan(c, "pg_get_tasks")
	defer span.End()

	el := entity.TaskSteps{}

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return nil, err
	}

	defer conn.Release()

	q := `
	SELECT vs.step_name, vs.time, vs.content 
	FROM pipeliner.variable_storage vs 
	WHERE work_id = $1
	ORDER BY vs.time DESC;`

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
		el = append(el, &s)
	}

	return el, nil
}
