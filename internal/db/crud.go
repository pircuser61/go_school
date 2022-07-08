package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.opencensus.io/trace"

	"github.com/google/uuid"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/configs"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

var (
	NullUuid = [16]byte{}
)

type PGConnection struct {
	Pool *pgxpool.Pool
}

func ConnectPostgres(ctx context.Context, db *configs.Database) (PGConnection, error) {
	maxConnections := strconv.Itoa(db.MaxConnections)
	connString := "postgres://" + db.User + ":" + db.Pass + "@" + db.Host + ":" + db.Port + "/" + db.DBName +
		"?sslmode=disable&pool_max_conns=" + maxConnections

	ctx, cancel := context.WithTimeout(ctx, time.Duration(db.Timeout)*time.Second)
	_ = cancel // no needed yet

	conn, err := pgxpool.Connect(ctx, connString)
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
	RunStatusStopped  int = 4
	RunStatusCreated  int = 5

	// language=PostgreSQL
	qCheckTagIsAttached string = `
		SELECT COUNT(pipeline_id) AS count
		FROM pipeliner.pipeline_tags    
		WHERE 
			pipeline_id = $1 and tag_id = $2`

	// language=PostgreSQL
	qWriteHistory = `
		INSERT INTO pipeliner.pipeline_history (
			id, 
			pipeline_id, 
			version_id, 
			date
		) 
		VALUES (
			$1, 
			$2, 
			$3, 
			$4
		)`
)

var (
	errCantFindPipelineVersion = errors.New("can't find pipeline version")
	errCantFindTag             = errors.New("can't find tag")
)

// TODO ErrNoRows ? Split file?

func parseRowsVersionList(c context.Context, rows pgx.Rows) ([]entity.EriusScenarioInfo, error) {
	_, span := trace.StartSpan(c, "parse_row_version_list")
	defer span.End()

	defer rows.Close()

	versionInfoList := make([]entity.EriusScenarioInfo, 0)

	for rows.Next() {
		e := entity.EriusScenarioInfo{}

		var approver sql.NullString

		err := rows.Scan(
			&e.VersionID,
			&e.Status,
			&e.ID,
			&e.CreatedAt,
			&e.Author,
			&approver,
			&e.Name,
			&e.LastRun,
			&e.LastRunStatus,
			&e.CommentRejected,
			&e.Comment,
		)
		if err != nil {
			return nil, err
		}

		e.Approver = approver.String

		versionInfoList = append(versionInfoList, e)
	}

	return versionInfoList, nil
}

func parseRowsVersionHistoryList(c context.Context, rows pgx.Rows) ([]entity.EriusVersionInfo, error) {
	_, span := trace.StartSpan(c, "parse_row_version_history_list")
	defer span.End()

	defer rows.Close()

	versionHistoryList := make([]entity.EriusVersionInfo, 0)

	for rows.Next() {
		e := entity.EriusVersionInfo{}

		var approver sql.NullString

		err := rows.Scan(
			&e.VersionID,
			&e.CreatedAt,
			&e.Author,
			&approver,
			&e.ApprovedAt,
		)
		if err != nil {
			return nil, err
		}

		e.Approver = approver.String

		versionHistoryList = append(versionHistoryList, e)
	}

	return versionHistoryList, nil
}

func (db *PGConnection) GetApprovedVersions(c context.Context) ([]entity.EriusScenarioInfo, error) {
	c, span := trace.StartSpan(c, "pg_get_approved_versions")
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

		version.ApprovedAt = &t

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

	for i := range final {
		vs := final[i]

		versionHistory, err := db.getVersionHistory(c, vs.ID)
		if err != nil {
			return nil, err
		}

		final[i].History = versionHistory
		n++
	}

	return final, nil
}

func (db *PGConnection) findApproveDate(c context.Context, id uuid.UUID) (time.Time, error) {
	c, span := trace.StartSpan(c, "pg_find_approve_time")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := `
		SELECT date 
		FROM pipeliner.pipeline_history 
		WHERE version_id = $1 
		ORDER BY date DESC
		LIMIT 1`

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

	// TODO pv.last_run_id isn't exist

	// nolint:gocritic
	// language=PostgreSQL
	q := `
	SELECT 
		pv.id, 
		pv.status, 
		pv.pipeline_id, 
		pv.created_at, 
		pv.author, 
		pv.approver, 
		pp.name, 
		pw.started_at, 
		pws.name, 
		pv.comment_rejected, 
		pv.comment
	FROM pipeliner.versions pv
	JOIN pipeliner.pipelines pp ON pv.pipeline_id = pp.id
	LEFT OUTER JOIN  pipeliner.works pw ON pw.id = pv.last_run_id
	LEFT OUTER JOIN  pipeliner.work_status pws ON pws.id = pw.status
	WHERE 
		pv.status = $1
		AND pp.deleted_at IS NULL
	ORDER BY created_at`

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
	c, span := trace.StartSpan(c, "pg_get_draft_versions")
	defer span.End()

	return db.GetVersionsByStatus(c, StatusDraft)
}

func (db *PGConnection) GetOnApproveVersions(c context.Context) ([]entity.EriusScenarioInfo, error) {
	c, span := trace.StartSpan(c, "pg_get_on_approve_versions")
	defer span.End()

	return db.GetVersionsByStatus(c, StatusOnApprove)
}

func (db *PGConnection) GetRejectedVersions(c context.Context) ([]entity.EriusScenarioInfo, error) {
	c, span := trace.StartSpan(c, "pg_get_rejected_versions")
	defer span.End()

	return db.GetVersionsByStatus(c, StatusRejected)
}

func (db *PGConnection) GetWorkedVersions(ctx context.Context) ([]entity.EriusScenario, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_worked_versions")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := `
	SELECT 
		pv.id, 
		pp.name, 
		pv.status, 
		pv.pipeline_id, 
		pv.content
	FROM pipeliner.versions pv
	JOIN pipeliner.pipelines pp ON pv.pipeline_id = pp.id
	WHERE 
		pv.status <> $1
	AND pp.deleted_at IS NULL
	ORDER BY pv.created_at`

	rows, err := db.Pool.Query(ctx, q, StatusDeleted)
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
	c, span := trace.StartSpan(c, "pg_get_all_tags")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return nil, err
	}

	defer conn.Release()

	// nolint:gocritic
	// language=PostgreSQL
	q := `SELECT 
		t.id, 
		t.name, 
		t.status, 
		t.color
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
	c, span := trace.StartSpan(c, "pg_get_pipeline_tag")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return nil, err
	}

	defer conn.Release()

	// nolint:gocritic
	// language=PostgreSQL
	q := `SELECT 
		t.id, 
		t.name, 
		t.status, 
		t.color, 
		t.is_marker
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

	// nolint:gocritic
	// language=PostgreSQL
	qSetApproved := `
		UPDATE pipeliner.versions 
		SET 
			status = $1, 
			approver = $2 
		WHERE id = $3`

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
		_ = tx.Rollback(c)

		return err
	}

	return nil
}

func (db *PGConnection) RollbackVersion(c context.Context, pipelineID, versionID uuid.UUID) error {
	c, span := trace.StartSpan(c, "pg_rollback_version")
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

	_, err = tx.Exec(c, qWriteHistory, id, pipelineID, versionID, date)
	if err != nil {
		return err
	}

	err = tx.Commit(c)
	if err != nil {
		_ = tx.Rollback(c)

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

	// nolint:gocritic
	// language=PostgreSQL
	qSetRejected := `
		UPDATE pipeliner.versions 
		SET 
			status = $1, 
			approver = $2, 
			comment_rejected = $3 
		WHERE id = $4`

	_, err = conn.Exec(c, qSetRejected, StatusRejected, author, comment, versionID)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGConnection) VersionEditable(c context.Context, versionID uuid.UUID) (bool, error) {
	c, span := trace.StartSpan(c, "pg_version_editable")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := `
		SELECT COUNT(id) AS count
		FROM pipeliner.versions 
		WHERE 
			id = $1 AND status = $2`

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

	// nolint:gocritic
	// language=PostgreSQL
	q := `
		SELECT COUNT(id) AS count
		FROM pipeliner.versions 
		WHERE pipeline_id = $1`

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

	// nolint:gocritic
	// language=PostgreSQL
	q := `
		SELECT COUNT(id) AS count
		FROM pipeliner.versions 
		WHERE 
			pipeline_id = $1 AND 
			author = $2 AND 
			(status = $3 OR status = $4 OR status = $5)`

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

	// nolint:gocritic
	// language=PostgreSQL
	qNewPipeline := `
	INSERT INTO pipeliner.pipelines (
		id, 
		name, 
		created_at, 
		author
	)
	VALUES (
		$1, 
		$2, 
		$3, 
		$4
	)`

	_, err = tx.Exec(c, qNewPipeline, p.ID, p.Name, createdAt, author)
	if err != nil {
		return err
	}

	err = tx.Commit(c)
	if err != nil {
		_ = tx.Rollback(c)

		return err
	}

	return db.CreateVersion(c, p, author, pipelineData)
}

func (db *PGConnection) CreateVersion(c context.Context,
	p *entity.EriusScenario, author string, pipelineData []byte) error {
	c, span := trace.StartSpan(c, "pg_create_version")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	qNewVersion := `
	INSERT INTO pipeliner.versions (
		id, 
		status, 
		pipeline_id, 
		created_at, 
		content, 
		author, 
		comment
	)
	VALUES (
		$1, 
		$2, 
		$3, 
		$4, 
		$5, 
		$6, 
		$7
	)`

	createdAt := time.Now()

	_, err := db.Pool.Exec(c, qNewVersion, p.VersionID, StatusDraft, p.ID, createdAt, pipelineData, author, p.Comment)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGConnection) PipelineNameCreatable(c context.Context, name string) (bool, error) {
	c, span := trace.StartSpan(c, "pg_pipeline_name_creatable")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return false, err
	}

	defer conn.Release()

	// nolint:gocritic
	// language=PostgreSQL
	q := `
	SELECT count(name) AS count
	FROM pipeliner.pipelines
	WHERE name = $1`

	row := conn.QueryRow(c, q, name)

	count := 0

	err = row.Scan(&count)
	if err != nil {
		return false, err
	}

	if count != 0 {
		return false, nil
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

	// nolint:gocritic
	// language=PostgreSQL
	qCheckTagExisted := `
	SELECT 
		t.id, 
		t.name, 
		t.status, 
		t.color, 
		t.is_marker
	FROM pipeliner.tags t
	WHERE 
		lower(t.name) = lower($1) AND t.status <> $2 and t.is_marker <> $3
	LIMIT 1`

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

	// nolint:gocritic
	// language=PostgreSQL
	qNewTag := `
		INSERT INTO pipeliner.tags (
			id, 
			name, 
			status,
			author,
			color,
			is_marker
		)
		VALUES (
			$1, 
			$2, 
			$3, 
			$4, 
			$5, 
			$6
		)
		RETURNING 
			id, 
			name, 
			status, 
			color, 
			is_marker`

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

	// nolint:gocritic
	// language=PostgreSQL
	q := `
		UPDATE pipeliner.versions 
		SET 
			deleted_at = $1, 
			status = $2 
		WHERE id = $3`
	t := time.Now()

	_, err := db.Pool.Exec(c, q, t, StatusDeleted, versionID)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGConnection) DeleteAllVersions(c context.Context, id uuid.UUID) error {
	c, span := trace.StartSpan(c, "pg_delete_all_versions")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := `
		UPDATE pipeliner.versions 
		SET 
			deleted_at = $1, 
			status = $2 
		WHERE pipeline_id = $3`
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

	// nolint:gocritic
	// language=PostgreSQL
	qName := `
		SELECT name 
		FROM pipeliner.pipelines 
		WHERE id = $1`
	row := db.Pool.QueryRow(c, qName, id)

	var n string

	err := row.Scan(&n)
	if err != nil {
		return err
	}

	n = n + "_deleted_at_" + t.String()

	// nolint:gocritic
	// language=PostgreSQL
	q := `
		UPDATE pipeliner.pipelines 
		SET 
			deleted_at = $1, 
			name = $2 
		WHERE id = $3`

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
	// nolint:gocritic
	// language=PostgreSQL
	q := `
	SELECT 
		pv.id, 
		pv.status, 
		pv.pipeline_id, 
		pv.content, 
		pv.comment
	FROM pipeliner.versions pv
	JOIN pipeliner.pipeline_history pph ON pph.version_id = pv.id
	WHERE pv.pipeline_id = $1
	ORDER BY pph.date DESC 
	LIMIT 1
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

	// nolint:gocritic
	// language=PostgreSQL
	qVersion := `
	SELECT 
		pv.id, 
		pv.status, 
		pv.pipeline_id, 
		pv.created_at, 
		pv.content, 
		pv.comment_rejected, 
		pv.comment, 
		pv.author, 
		pph.date
	FROM pipeliner.versions pv
    LEFT JOIN pipeliner.pipeline_history pph ON pph.version_id = pv.id
	WHERE pv.id = $1
	ORDER BY pph.date DESC 
	LIMIT 1`

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
			d        *time.Time
			ca       *time.Time
			a        string
		)

		err := rows.Scan(&vID, &s, &pID, &ca, &c, &cr, &cm, &a, &d)
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
		p.ApprovedAt = d
		p.CreatedAt = ca
		p.Author = a

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

	// nolint:gocritic
	// language=PostgreSQL
	qGetTag := `
	SELECT 
		t.id, 
		t.name, 
		t.status, 
		t.color, 
		t.is_marker
	FROM pipeliner.tags t
	WHERE 
		t.id = $1 AND t.status <> $2
	LIMIT 1`

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

	// nolint:gocritic
	// language=PostgreSQL
	qCheckTagIsCreated := `
		SELECT count(id) AS count
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

	// nolint:gocritic
	// language=PostgreSQL
	qEditTag := `UPDATE pipeliner.tags
	SET color = $1
	WHERE id = $2`

	_, err = conn.Exec(c, qEditTag, e.Color, e.ID)
	if err != nil {
		return err
	}

	return nil
}

//nolint:dupl //its different
func (db *PGConnection) AttachTag(c context.Context, pid uuid.UUID, e *entity.EriusTagInfo) error {
	c, span := trace.StartSpan(c, "pg_attach_tag")
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

	// nolint:gocritic
	// language=PostgreSQL
	qAttachTag := `
	INSERT INTO pipeliner.pipeline_tags (
		pipeline_id, 
		tag_id
	)
	VALUES (
		$1, 
		$2
	)`

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

	// nolint:gocritic
	// language=PostgreSQL
	qName := `SELECT 
		name 
	FROM pipeliner.tags 
	WHERE id = $1`

	row := conn.QueryRow(c, qName, id)

	var n string

	err = row.Scan(&n)
	if err != nil {
		return err
	}

	n = n + "_deleted_at_" + t.String()

	// nolint:gocritic
	// language=PostgreSQL
	qSetTagDeleted := `UPDATE pipeliner.tags
	SET 
		status = $1, 
		name = $2  
	WHERE id = $3`

	_, err = conn.Exec(c, qSetTagDeleted, StatusDeleted, n, id)
	if err != nil {
		return err
	}

	// nolint:gocritic
	// language=PostgreSQL
	qCheckTagAttached := `
	SELECT COUNT(pipeline_id) AS count
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

	// nolint:gocritic
	// language=PostgreSQL
	qRemoveAttachedTags := `
	DELETE FROM pipeliner.pipeline_tags
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

	// nolint:gocritic
	// language=PostgreSQL
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

	// nolint:gocritic
	// language=PostgreSQL
	qCheckTagIsAttached := `
	SELECT COUNT(pipeline_id) AS count
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

	// nolint:gocritic
	// language=PostgreSQL
	qRemovePipelineTags := `
	DELETE FROM pipeliner.pipeline_tags
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

	// nolint:gocritic
	// language=PostgreSQL
	q := `
	UPDATE pipeliner.versions 
	SET 
		status = $1, 
		content = $2, 
		comment = $3 
	WHERE id = $4`

	_, err := db.Pool.Exec(c, q, p.Status, pipelineData, p.Comment, p.VersionID)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGConnection) SaveStepContext(ctx context.Context, dto *SaveStepRequest) (uuid.UUID, time.Time, error) {
	ctx, span := trace.StartSpan(ctx, "pg_save_step_context")
	defer span.End()

	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return NullUuid, time.Time{}, err
	}

	defer conn.Release()

	var id uuid.UUID
	var t time.Time

	q := `
	SELECT id, time
	FROM pipeliner.variable_storage 
	WHERE work_id = $1 AND step_name = $2 AND status IN ('idle', 'ready', 'running')
`
	if scanErr := conn.QueryRow(ctx, q,
		dto.WorkID,
		dto.StepName).Scan(&id, &t); scanErr != nil && !errors.Is(scanErr, pgx.ErrNoRows) {
		return NullUuid, time.Time{}, nil
	}
	if id != NullUuid {
		return id, t, nil
	}

	id = uuid.New()
	timestamp := time.Now()
	// nolint:gocritic
	// language=PostgreSQL
	q = `
	INSERT INTO pipeliner.variable_storage (
		id, 
		work_id, 
		step_type,
		step_name, 
		content, 
		time, 
		break_points, 
		has_error,
		status
	)
	VALUES (
		$1, 
		$2, 
		$3, 
		$4, 
		$5, 
		$6, 
		$7,
	    $8,
	    $9
	)
`

	_, err = conn.Exec(
		ctx,
		q,
		id,
		dto.WorkID,
		dto.StepType,
		dto.StepName,
		dto.Content,
		timestamp,
		dto.BreakPoints,
		dto.HasError,
		dto.Status,
	)
	if err != nil {
		return NullUuid, time.Time{}, err
	}

	return id, timestamp, nil
}

func (db *PGConnection) UpdateStepContext(ctx context.Context, dto *UpdateStepRequest) error {
	c, span := trace.StartSpan(ctx, "pg_update_step_context")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return err
	}

	defer conn.Release()

	// nolint:gocritic
	// language=PostgreSQL
	q := `
	UPDATE pipeliner.variable_storage
	SET
	    break_points = $2
		, has_error = $3
	    , status = $4
	    --content--
	WHERE
		id = $1
`
	args := []interface{}{dto.Id, dto.BreakPoints, dto.HasError, dto.Status}
	if !dto.WithoutContent {
		q = strings.Replace(q, "--content--", ", content = $5", -1)
		args = append(args, dto.Content)
	}

	_, err = conn.Exec(
		c,
		q,
		args...,
	)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGConnection) CreateTask(c context.Context,
	taskID, versionID uuid.UUID, author string, isDebugMode bool, parameters []byte) (*entity.EriusTask, error) {
	c, span := trace.StartSpan(c, "pg_create_task")
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
	// nolint:gocritic
	// language=PostgreSQL
	q := `
	INSERT INTO pipeliner.works(
		id, 
		version_id, 
		started_at, 
		status, 
		author, 
		debug, 
		parameters
	)
	VALUES (
		$1, 
		$2, 
		$3, 
		$4, 
		$5, 
		$6, 
		$7
	)
	RETURNING work_number
`

	row := tx.QueryRow(c, q, taskID, versionID, startedAt, RunStatusCreated, author, isDebugMode, parameters)

	var worksNumber string

	err = row.Scan(&worksNumber)
	if err != nil {
		_ = tx.Rollback(c)

		return nil, err
	}

	// TODO pv.last_run_id isn't exist
	// nolint:gocritic
	// language=PostgreSQL
	q = `UPDATE pipeliner.versions 
		SET last_run_id = $1 
		WHERE id = $2`

	_, err = tx.Exec(c, q, taskID, versionID)
	if err != nil {
		_ = tx.Rollback(c)

		return nil, err
	}

	err = tx.Commit(c)
	if err != nil {
		_ = tx.Rollback(c)

		return nil, err
	}

	return db.GetTask(c, worksNumber)
}

func (db *PGConnection) ChangeTaskStatus(c context.Context,
	taskID uuid.UUID, status int) error {
	c, span := trace.StartSpan(c, "pg_change_task_status")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return err
	}

	defer conn.Release()

	var q string
	// nolint:gocritic
	// language=PostgreSQL
	if status == RunStatusFinished {
		q = `UPDATE pipeliner.works 
		SET status = $1, finished_at = now()
		WHERE id = $2`
	} else {
		q = `UPDATE pipeliner.works 
		SET status = $1
		WHERE id = $2`
	}

	_, err = conn.Exec(c, q, status, taskID)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGConnection) GetExecutableScenarios(c context.Context) ([]entity.EriusScenario, error) {
	c, span := trace.StartSpan(c, "pg_get_executable_scenarios")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := `
	SELECT 
		pv.id, 
		pp.name, 
		pv.status, 
		pv.pipeline_id, 
		pv.content, 
		ph.date
	FROM pipeliner.versions pv
	JOIN pipeliner.pipelines pp ON pv.pipeline_id = pp.id
	JOIN pipeliner.pipeline_history ph ON ph.version_id = pv.id
	WHERE 
		pv.status = $1
		AND pp.deleted_at is NULL
	ORDER BY pv.created_at`

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
		p.ApprovedAt = &d
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

			if finV.ApprovedAt.After(t) {
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
	c, span := trace.StartSpan(c, "pg_get_executable_by_name")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return nil, err
	}

	defer conn.Release()

	p := entity.EriusScenario{}
	// nolint:gocritic
	// language=PostgreSQL
	q := `
	SELECT 
		pv.id, 
		pv.status, 
		pv.pipeline_id, 
		pv.content
	FROM pipeliner.versions pv
	JOIN pipeliner.pipeline_history pph on pph.version_id = pv.id
	JOIN pipeliner.pipelines p on p.id = pv.pipeline_id
	WHERE 
		p.name = $1 
		AND p.deleted_at IS NULL
	ORDER BY pph.date DESC 
	LIMIT 1
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

//TODO:last_updated_at
//nolint:gocritic //filters
func compileGetTasksQuery(filters entity.TaskFilter) (q string, args []interface{}) {
	// nolint:gocritic
	// language=PostgreSQL
	q = `
		SELECT 
			w.id,
			w.started_at,
       		w.started_at,
			ws.name, 
			w.human_status, 
			w.debug, 
			w.parameters, 
			w.author, 
			w.version_id,
       		w.work_number,
       		p.name
		FROM pipeliner.works w 
		JOIN pipeliner.versions v ON v.id = w.version_id
		JOIN pipeliner.pipelines p ON p.id = v.pipeline_id
		JOIN pipeliner.work_status ws ON w.status = ws.id
		LEFT JOIN LATERAL (
             SELECT * FROM pipeliner.variable_storage vs
             WHERE vs.work_id = w.id
             ORDER BY vs.time DESC
             LIMIT 1
        ) workers ON workers.work_id = w.id
		WHERE 1=1`

	order := "ASC"
	if filters.Order != nil {
		order = *filters.Order
	}

	args = append(args, filters.CurrentUser)
	if filters.SelectAs != nil {
		switch *filters.SelectAs {
		case "approver":
			{
				q = fmt.Sprintf("%s AND workers.content::json->'State'->workers.step_name->'approvers'->$%d "+
					"IS NOT NULL AND workers.status != 'finished'", q, len(args))
			}
		case "executor":
			{
				q = fmt.Sprintf("%s AND workers.content::json->'State'->workers.step_name->'executors'->$%d "+
					"IS NOT NULL AND (workers.status != 'finished' AND workers.status != 'no_success')", q, len(args))
			}
		case "finished_executor":
			{
				q = strings.Replace(q, "LIMIT 1", "", -1)
				q = fmt.Sprintf("%s AND workers.content::json->'State'->workers.step_name->'executors'->$%d "+
					"IS NOT NULL AND (workers.status = 'finished' OR workers.status = 'no_success')", q, len(args))
			}
		}
	} else {
		q = fmt.Sprintf("%s AND w.author = $%d", q, len(args))
	}

	if filters.TaskIDs != nil {
		args = append(args, filters.TaskIDs)
		q = fmt.Sprintf("%s AND w.work_number = ANY($%d)", q, len(args))
	}
	if filters.Name != nil {
		args = append(args, *filters.Name)
		q = fmt.Sprintf("%s AND p.name ILIKE $%d || '%%'", q, len(args))
	}
	if filters.Created != nil {
		args = append(args, time.Unix(int64(filters.Created.Start), 0).UTC(), time.Unix(int64(filters.Created.End), 0).UTC())
		q = fmt.Sprintf("%s AND w.started_at BETWEEN $%d AND $%d", q, len(args)-1, len(args))
	}
	if filters.Archived != nil {
		switch *filters.Archived {
		case true:
			q = fmt.Sprintf("%s AND (now()::TIMESTAMP - w.finished_at::TIMESTAMP) > '3 days'", q)
		case false:
			q = fmt.Sprintf("%s AND ((now()::TIMESTAMP - w.finished_at::TIMESTAMP) < '3 days' OR w.finished_at IS NULL)", q)
		}
	}
	if order != "" {
		q = fmt.Sprintf("%s\n ORDER BY w.started_at %s", q, order)
	}
	if filters.Offset != nil {
		args = append(args, *filters.Offset)
		q = fmt.Sprintf("%s\n OFFSET $%d", q, len(args))
	}
	if filters.Limit != nil {
		args = append(args, *filters.Limit)
		q = fmt.Sprintf("%s\n LIMIT $%d", q, len(args))
	}

	return q, args
}

func (db *PGConnection) UpdateTaskHumanStatus(c context.Context, taskID uuid.UUID, status string) error {
	c, span := trace.StartSpan(c, "update_task_status")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return err
	}
	defer conn.Release()

	// nolint:gocritic
	// language=PostgreSQL
	q := `UPDATE pipeliner.works
		SET human_status = $1
		WHERE id = $2`

	_, err = conn.Exec(c, q, status, taskID)
	return err
}

//nolint:gocritic //filters
func (db *PGConnection) GetTasks(c context.Context, filters entity.TaskFilter) (*entity.EriusTasksPage, error) {
	c, span := trace.StartSpan(c, "pg_get_tasks")
	defer span.End()

	q, args := compileGetTasksQuery(filters)

	tasks, err := db.getTasks(c, q, args)
	if err != nil {
		return nil, err
	}

	filters.Limit = nil
	filters.Offset = nil
	q, args = compileGetTasksQuery(filters)

	count, err := db.getTasksCount(c, q, args)
	if err != nil {
		return nil, err
	}

	return &entity.EriusTasksPage{
		Tasks: tasks.Tasks,
		Total: count,
	}, nil
}

func (db *PGConnection) GetPipelineTasks(c context.Context, pipelineID uuid.UUID) (*entity.EriusTasks, error) {
	c, span := trace.StartSpan(c, "pg_get_pipeline_tasks")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := `SELECT 
			w.id, 
			w.started_at, 
			ws.name, 
			w.human_status, 
			w.debug, 
			w.parameters, 
			w.author, 
			w.version_id,
       		w.work_number
		FROM pipeliner.works w 
		JOIN pipeliner.versions v ON v.id = w.version_id
		JOIN pipeliner.pipelines p ON p.id = v.pipeline_id
		JOIN pipeliner.work_status ws ON w.status = ws.id
		WHERE p.id = $1
		ORDER BY w.started_at DESC
		LIMIT 100`

	return db.getTasks(c, q, []interface{}{pipelineID})
}

func (db *PGConnection) GetVersionTasks(c context.Context, versionID uuid.UUID) (*entity.EriusTasks, error) {
	c, span := trace.StartSpan(c, "pg_get_version_tasks")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := `SELECT 
			w.id, 
			w.started_at, 
			ws.name,
       		w.human_status,
			w.debug, 
			w.parameters,
			w.author, 
			w.version_id,
       		w.work_number
		FROM pipeliner.works w 
		JOIN pipeliner.versions v ON v.id = w.version_id
		JOIN pipeliner.work_status ws ON w.status = ws.id
		WHERE v.id = $1
		ORDER BY w.started_at DESC
		LIMIT 100`

	return db.getTasks(c, q, []interface{}{versionID})
}

func (db *PGConnection) GetLastDebugTask(c context.Context, id uuid.UUID, author string) (*entity.EriusTask, error) {
	c, span := trace.StartSpan(c, "pg_get_last_debug_task")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := `SELECT 
			w.id, 
			w.started_at, 
			ws.name, 
       		w.human_status,
			w.debug, 
			w.parameters, 
			w.author, 
			w.version_id
		FROM pipeliner.works w 
		JOIN pipeliner.versions v ON v.id = w.version_id
		JOIN pipeliner.work_status ws ON w.status = ws.id
		WHERE v.id = $1
		AND w.author = $2
		AND w.debug = true
		ORDER BY w.started_at DESC
		LIMIT 1`

	et := entity.EriusTask{}

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return nil, err
	}

	defer conn.Release()

	row := conn.QueryRow(c, q, id, author)
	parameters := ""

	err = row.Scan(&et.ID, &et.StartedAt, &et.Status, &et.HumanStatus, &et.IsDebugMode, &parameters, &et.Author, &et.VersionID)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal([]byte(parameters), &et.Parameters)
	if err != nil {
		return nil, err
	}

	return &et, nil
}

func (db *PGConnection) GetTask(c context.Context, workNumber string) (*entity.EriusTask, error) {
	c, span := trace.StartSpan(c, "pg_get_task")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	const q = `SELECT 
			w.id, 
			w.started_at, 
			w.started_at, 
			ws.name,
       		w.human_status,
			w.debug, 
			COALESCE(w.parameters, '{}') AS parameters,
			w.author,
			w.version_id,
			w.work_number,
			p.name
		FROM pipeliner.works w 
		JOIN pipeliner.versions v ON v.id = w.version_id
		JOIN pipeliner.pipelines p ON p.id = v.pipeline_id
		JOIN pipeliner.work_status ws ON w.status = ws.id
		WHERE w.work_number = $1`

	return db.getTask(c, q, workNumber)
}

func (db *PGConnection) getTask(c context.Context, q, workNumber string) (*entity.EriusTask, error) {
	c, span := trace.StartSpan(c, "pg_get_task_private")
	defer span.End()

	et := entity.EriusTask{}

	var nullStringParameters sql.NullString

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return nil, err
	}

	defer conn.Release()

	row := conn.QueryRow(c, q, workNumber)

	err = row.Scan(
		&et.ID,
		&et.StartedAt,
		&et.LastChangedAt,
		&et.Status,
		&et.HumanStatus,
		&et.IsDebugMode,
		&nullStringParameters,
		&et.Author,
		&et.VersionID,
		&et.WorkNumber,
		&et.Name,
	)
	if err != nil {
		return nil, err
	}

	if nullStringParameters.Valid && nullStringParameters.String != "" {
		err = json.Unmarshal([]byte(nullStringParameters.String), &et.Parameters)
		if err != nil {
			return nil, err
		}
	}

	return &et, nil
}

func (db *PGConnection) getTasksCount(c context.Context, q string, args []interface{}) (int, error) {
	c, span := trace.StartSpan(c, "pg_get_tasks_count")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return -1, err
	}

	defer conn.Release()

	q = fmt.Sprintf("SELECT COUNT(*) FROM (%s) sub", q)

	var count int
	if scanErr := conn.QueryRow(c, q, args...).Scan(&count); scanErr != nil {
		return -1, scanErr
	}
	return count, nil
}

func (db *PGConnection) getTasks(c context.Context, q string, args []interface{}) (*entity.EriusTasks, error) {
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

	rows, err := conn.Query(c, q, args...)
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
			&et.LastChangedAt,
			&et.Status,
			&et.HumanStatus,
			&et.IsDebugMode,
			&nullStringParameters,
			&et.Author,
			&et.VersionID,
			&et.WorkNumber,
			&et.Name)

		if err != nil {
			return nil, err
		}

		if nullStringParameters.Valid && nullStringParameters.String != "" {
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
	c, span := trace.StartSpan(c, "pg_get_task_steps")
	defer span.End()

	el := entity.TaskSteps{}

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return nil, err
	}

	defer conn.Release()

	// nolint:gocritic
	// language=PostgreSQL
	q := `
	SELECT 
	    vs.id,
	    vs.step_type,
		vs.step_name, 
		vs.time, 
		vs.content, 
		COALESCE(vs.break_points, '{}') AS break_points, 
		vs.has_error,
		vs.status
	FROM pipeliner.variable_storage vs 
	WHERE work_id = $1
	ORDER BY vs.time DESC`

	rows, err := conn.Query(c, q, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	//nolint:dupl //scan
	for rows.Next() {
		s := entity.Step{}
		var content string

		err = rows.Scan(
			&s.ID,
			&s.Type,
			&s.Name,
			&s.Time,
			&content,
			&s.BreakPoints,
			&s.HasError,
			&s.Status,
		)
		if err != nil {
			return nil, err
		}

		storage := store.NewStore()

		err = json.Unmarshal([]byte(content), storage)
		if err != nil {
			return nil, err
		}

		s.State = storage.State
		s.Steps = storage.Steps
		s.Errors = storage.Errors
		s.Storage = storage.Values
		el = append(el, &s)
	}

	return el, nil
}

func (db *PGConnection) GetUnfinishedTaskStepsByWorkIdAndStepType(
	ctx context.Context,
	id uuid.UUID,
	stepType string,
) (entity.TaskSteps, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_unfinished_task_steps_by_work_id_and_step_type")
	defer span.End()

	el := entity.TaskSteps{}

	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}

	defer conn.Release()

	// nolint:gocritic
	// language=PostgreSQL
	q := `
	SELECT 
	    vs.id,
	    vs.step_type,
		vs.step_name, 
		vs.time, 
		vs.content, 
		COALESCE(vs.break_points, '{}') AS break_points, 
		vs.has_error,
		vs.status
	FROM pipeliner.variable_storage vs 
	WHERE 
	    work_id = $1 AND 
	    step_type = $2 AND
	    status != 'finished'
	ORDER BY vs.time ASC`

	rows, err := conn.Query(ctx, q, id, stepType)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	//nolint:dupl //scan
	for rows.Next() {
		s := entity.Step{}
		var content string

		err = rows.Scan(
			&s.ID,
			&s.Type,
			&s.Name,
			&s.Time,
			&content,
			&s.BreakPoints,
			&s.HasError,
			&s.Status,
		)
		if err != nil {
			return nil, err
		}

		storage := store.NewStore()

		err = json.Unmarshal([]byte(content), storage)
		if err != nil {
			return nil, err
		}

		s.State = storage.State
		s.Steps = storage.Steps
		s.Errors = storage.Errors
		s.Storage = storage.Values
		el = append(el, &s)
	}

	return el, nil
}

func (db *PGConnection) GetTaskStepById(ctx context.Context, id uuid.UUID) (*entity.Step, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_task_step")
	defer span.End()

	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}

	defer conn.Release()

	// nolint:gocritic
	// language=PostgreSQL
	q := `
	SELECT 
	    vs.id,
	    vs.step_type,
		vs.step_name, 
		vs.time, 
		vs.content, 
		COALESCE(vs.break_points, '{}') AS break_points, 
		vs.has_error,
		vs.status
	FROM pipeliner.variable_storage vs 
	WHERE id = $1
	LIMIT 1`

	var s entity.Step
	var content string
	err = conn.QueryRow(ctx, q, id).Scan(
		&s.ID,
		&s.Type,
		&s.Name,
		&s.Time,
		&content,
		&s.BreakPoints,
		&s.HasError,
		&s.Status,
	)
	if err != nil {
		return nil, err
	}

	storage := store.NewStore()

	err = json.Unmarshal([]byte(content), storage)
	if err != nil {
		return nil, err
	}

	s.State = storage.State
	s.Steps = storage.Steps
	s.Errors = storage.Errors
	s.Storage = storage.Values

	return &s, nil
}

func (db *PGConnection) getVersionHistory(c context.Context, id uuid.UUID) ([]entity.EriusVersionInfo, error) {
	c, span := trace.StartSpan(c, "pg_get_version_history")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return nil, err
	}

	defer conn.Release()

	// nolint:gocritic
	// language=PostgreSQL
	q := `
	SELECT 
		pv.id, 
		pv.created_at, 
		pv.author, 
		pv.approver, 
		ph.date
	FROM pipeliner.versions pv
	JOIN pipeliner.pipelines pp ON pv.pipeline_id = pp.id
	LEFT OUTER JOIN pipeliner.pipeline_history ph ON pv.id = ph.version_id
	WHERE 
		pv.status = $1 
		AND pp.id = $2 
		AND pp.deleted_at IS NULL
	ORDER BY date`

	rows, err := conn.Query(c, q, StatusApproved, id)
	if err != nil {
		return nil, err
	}

	res, err := parseRowsVersionHistoryList(c, rows)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (db *PGConnection) GetVersionsByBlueprintID(c context.Context, bID string) ([]entity.EriusScenario, error) {
	c, span := trace.StartSpan(c, "pg_get_versions_by_blueprint_id")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return nil, err
	}

	defer conn.Release()

	// nolint:gocritic
	// language=PostgreSQL
	const query = `
	SELECT
		pv.id,
		pv.status,
		pv.pipeline_id,
		pv.created_at,
		pv.content,
		pv.comment_rejected,
		pv.comment,
		pv.author,
		(SELECT MAX(date) FROM pipeliner.pipeline_history WHERE pipeline_id = pv.pipeline_id) AS last_approve
	FROM (
			 SELECT servicedesk_node.id                                                                 AS pipeline_version_id,
					servicedesk_node.blocks -> servicedesk_node.nextNode ->> 'type_id'                  AS type_id,
					servicedesk_node.blocks -> servicedesk_node.nextNode -> 'params' ->> 'blueprint_id' AS blueprint_id
			 FROM (
					  SELECT id, blocks, nextNode
					  FROM (
							   SELECT id,
									  pipeline.blocks                                                               as blocks,
									  jsonb_array_elements_text(pipeline.blocks -> pipeline.entrypoint #> '{next,default}') as nextNode
							   FROM (
										SELECT id,
											   content -> 'pipeline' #> '{blocks}'    as blocks,
											   content -> 'pipeline' ->> 'entrypoint' as entrypoint
										FROM pipeliner.versions
									) as pipeline
						   ) as next_from_start
					  WHERE next_from_start.nextNode LIKE 'servicedesk_application%'
				  ) as servicedesk_node
	) as servicedesk_node_params
		LEFT JOIN pipeliner.versions pv ON pv.id = servicedesk_node_params.pipeline_version_id
	WHERE pv.status = 2 AND
			pv.created_at = (SELECT MAX(v.created_at) FROM pipeliner.versions v WHERE v.pipeline_id = pv.pipeline_id AND v.status = 2) AND
			servicedesk_node_params.blueprint_id = $1 AND
			servicedesk_node_params.type_id = 'servicedesk_application';
`

	rows, err := conn.Query(c, query, bID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	res := make([]entity.EriusScenario, 0)

	if rows.Next() {
		var (
			vID, pID uuid.UUID
			s        int
			c        string
			cr       string
			cm       string
			d        *time.Time
			ca       *time.Time
			a        string
		)

		err = rows.Scan(&vID, &s, &pID, &ca, &c, &cr, &cm, &a, &d)
		if err != nil {
			return nil, err
		}

		p := entity.EriusScenario{}

		err = json.Unmarshal([]byte(c), &p)
		if err != nil {
			return nil, err
		}

		p.VersionID = vID
		p.ID = pID
		p.Status = s
		p.CommentRejected = cr
		p.Comment = cm
		p.ApprovedAt = d
		p.CreatedAt = ca
		p.Author = a

		res = append(res, p)
	}

	return res, nil
}
