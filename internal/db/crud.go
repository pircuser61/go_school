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

	"github.com/lib/pq"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/configs"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

var (
	NullUuid = [16]byte{}
)

type Connector interface {
	Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, optionsAndArgs ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, optionsAndArgs ...interface{}) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

func (db *PGCon) StartTransaction(ctx context.Context) (Database, error) {
	tx, err := db.Connection.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &PGCon{Connection: tx}, nil
}

func (db *PGCon) CommitTransaction(ctx context.Context) error {
	tx, ok := db.Connection.(pgx.Tx)
	if !ok {
		return nil
	}
	return tx.Commit(ctx)
}

func (db *PGCon) RollbackTransaction(ctx context.Context) error {
	tx, ok := db.Connection.(pgx.Tx)
	if !ok {
		return nil
	}
	return tx.Rollback(ctx) // nolint:errcheck // rollback err
}

type PGCon struct {
	Connection Connector
}

func ConnectPostgres(ctx context.Context, db *configs.Database) (PGCon, error) {
	maxConnections := strconv.Itoa(db.MaxConnections)
	connString := "postgres://" + db.User + ":" + db.Pass + "@" + db.Host + ":" + db.Port + "/" + db.DBName +
		"?sslmode=disable&pool_max_conns=" + maxConnections

	ctx, cancel := context.WithTimeout(ctx, time.Duration(db.Timeout)*time.Second)
	_ = cancel // no needed yet

	conn, err := pgxpool.Connect(ctx, connString)
	if err != nil {
		return PGCon{}, err
	}

	pgc := PGCon{Connection: conn}

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
		FROM pipeline_tags    
		WHERE 
			pipeline_id = $1 and tag_id = $2`

	// language=PostgreSQL
	qWriteHistory = `
		INSERT INTO pipeline_history (
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
			&approver,
			&e.Author,
			&e.CreatedAt,
			&e.UpdatedAt,
			&e.IsActual,
			&e.Status,
		)
		if err != nil {
			return nil, err
		}

		e.Approver = approver.String

		versionHistoryList = append(versionHistoryList, e)
	}

	return versionHistoryList, nil
}

func (db *PGCon) GetPipelinesWithLatestVersion(c context.Context, author string) ([]entity.EriusScenarioInfo, error) {
	c, span := trace.StartSpan(c, "pg_get_pipelines_with_latest_version")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := `
SELECT pv.id,
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
FROM versions pv
         JOIN pipelines pp ON pv.pipeline_id = pp.id
         LEFT OUTER JOIN works pw ON pw.id = pv.last_run_id
         LEFT OUTER JOIN work_status pws ON pws.id = pw.status
WHERE pp.deleted_at IS NULL
  AND updated_at = (
    SELECT MAX(updated_at)
    FROM versions pv2
    WHERE pv.pipeline_id = pv2.pipeline_id
      AND pv2.status NOT IN (3, 4)
)
  ---author---
ORDER BY created_at;`

	fmt.Println("author: ", author)

	if author != "" {
		q = strings.ReplaceAll(q, "---author---", "AND pv.author='"+author+"'")
	}

	rows, err := db.Connection.Query(c, q)
	if err != nil {
		return nil, err
	}

	res, err := parseRowsVersionList(c, rows)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (db *PGCon) GetApprovedVersions(c context.Context) ([]entity.EriusScenarioInfo, error) {
	c, span := trace.StartSpan(c, "pg_get_approved_versions")
	defer span.End()

	vMap := make(map[uuid.UUID]entity.EriusScenarioInfo)

	versions, err := db.GetVersionsByStatus(c, StatusApproved, "")
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

	return final, nil
}

func (db *PGCon) GetPipelineVersions(c context.Context, id uuid.UUID) ([]entity.EriusVersionInfo, error) {
	c, span := trace.StartSpan(c, "pg_get_pipeline_versions")
	defer span.End()

	return db.getVersionHistory(c, id, -1)
}

func (db *PGCon) findApproveDate(c context.Context, id uuid.UUID) (time.Time, error) {
	c, span := trace.StartSpan(c, "pg_find_approve_time")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := `
		SELECT date 
		FROM pipeline_history 
		WHERE version_id = $1 
		ORDER BY date DESC
		LIMIT 1`

	rows, err := db.Connection.Query(c, q, id)
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

func (db *PGCon) GetVersionsByStatus(c context.Context, status int, author string) ([]entity.EriusScenarioInfo, error) {
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
		FROM versions pv
		JOIN pipelines pp ON pv.pipeline_id = pp.id
		LEFT OUTER JOIN  works pw ON pw.id = pv.last_run_id
		LEFT OUTER JOIN  work_status pws ON pws.id = pw.status
		WHERE 
			pv.status = $1
			AND pp.deleted_at IS NULL
			---author---
		ORDER BY created_at`

	if author != "" {
		q = strings.ReplaceAll(q, "---author---", "AND pv.author='"+author+"'")
	}

	rows, err := db.Connection.Query(c, q, status)
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

func (db *PGCon) GetDraftVersions(c context.Context, author string) ([]entity.EriusScenarioInfo, error) {
	c, span := trace.StartSpan(c, "pg_get_draft_versions")
	defer span.End()

	return db.GetVersionsByStatus(c, StatusDraft, author)
}

func (db *PGCon) GetOnApproveVersions(c context.Context) ([]entity.EriusScenarioInfo, error) {
	c, span := trace.StartSpan(c, "pg_get_on_approve_versions")
	defer span.End()

	return db.GetVersionsByStatus(c, StatusOnApprove, "")
}

func (db *PGCon) GetRejectedVersions(c context.Context) ([]entity.EriusScenarioInfo, error) {
	c, span := trace.StartSpan(c, "pg_get_rejected_versions")
	defer span.End()

	return db.GetVersionsByStatus(c, StatusRejected, "")
}

func (db *PGCon) GetWorkedVersions(ctx context.Context) ([]entity.EriusScenario, error) {
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
	FROM versions pv
	JOIN pipelines pp ON pv.pipeline_id = pp.id
	WHERE 
		pv.status <> $1
	AND pp.deleted_at IS NULL
	ORDER BY pv.created_at`

	rows, err := db.Connection.Query(ctx, q, StatusDeleted)
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

func (db *PGCon) GetAllTags(c context.Context) ([]entity.EriusTagInfo, error) {
	c, span := trace.StartSpan(c, "pg_get_all_tags")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := `SELECT 
		t.id, 
		t.name, 
		t.status, 
		t.color
	FROM tags t
    WHERE 
		t.status <> $1`

	rows, err := db.Connection.Query(c, q, StatusDeleted)
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

func (db *PGCon) GetPipelineTag(c context.Context, pid uuid.UUID) ([]entity.EriusTagInfo, error) {
	c, span := trace.StartSpan(c, "pg_get_pipeline_tag")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := `SELECT 
		t.id, 
		t.name, 
		t.status, 
		t.color, 
		t.is_marker
	FROM tags t
	LEFT OUTER JOIN pipeline_tags pt ON pt.tag_id = t.id
    WHERE 
		t.status <> $1 
	AND
		pt.pipeline_id = $2`

	rows, err := db.Connection.Query(c, q, StatusDeleted, pid)
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

func (db *PGCon) SwitchApproved(c context.Context, pipelineID, versionID uuid.UUID, author string) error {
	c, span := trace.StartSpan(c, "pg_switch_approved")
	defer span.End()

	date := time.Now()

	tx, err := db.Connection.Begin(c)
	if err != nil {
		return err
	}
	defer tx.Rollback(c) // nolint:errcheck // rollback err

	id := uuid.New()

	// nolint:gocritic
	// language=PostgreSQL
	qSetApproved := `
		UPDATE versions 
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

	return tx.Commit(c)
}

func (db *PGCon) RollbackVersion(c context.Context, pipelineID, versionID uuid.UUID) error {
	c, span := trace.StartSpan(c, "pg_rollback_version")
	defer span.End()

	date := time.Now()

	id := uuid.New()

	_, err := db.Connection.Exec(c, qWriteHistory, id, pipelineID, versionID, date)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGCon) SwitchRejected(c context.Context, versionID uuid.UUID, comment, author string) error {
	c, span := trace.StartSpan(c, "pg_switch_rejected")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	qSetRejected := `
		UPDATE versions 
		SET 
			status = $1, 
			approver = $2, 
			comment_rejected = $3 
		WHERE id = $4`

	_, err := db.Connection.Exec(c, qSetRejected, StatusRejected, author, comment, versionID)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGCon) VersionEditable(c context.Context, versionID uuid.UUID) (bool, error) {
	c, span := trace.StartSpan(c, "pg_version_editable")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := `
		SELECT COUNT(id) AS count
		FROM versions 
		WHERE 
			id = $1 AND status = $2`

	rows, err := db.Connection.Query(c, q, versionID, StatusApproved)
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

func (db *PGCon) PipelineRemovable(c context.Context, id uuid.UUID) (bool, error) {
	c, span := trace.StartSpan(c, "pg_pipeline_removable")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := `
		SELECT COUNT(id) AS count
		FROM versions 
		WHERE pipeline_id = $1`

	row := db.Connection.QueryRow(c, q, id)

	count := 0

	err := row.Scan(&count)
	if err != nil {
		return false, err
	}

	if count == 1 {
		return true, nil
	}

	return false, nil
}

func (db *PGCon) CreatePipeline(c context.Context,
	p *entity.EriusScenario, author string, pipelineData []byte) error {
	c, span := trace.StartSpan(c, "pg_create_pipeline")
	defer span.End()

	createdAt := time.Now()

	// nolint:gocritic
	// language=PostgreSQL
	qNewPipeline := `
	INSERT INTO pipelines (
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

	_, err := db.Connection.Exec(c, qNewPipeline, p.ID, p.Name, createdAt, author)
	if err != nil {
		return err
	}

	return db.CreateVersion(c, p, author, pipelineData)
}

func (db *PGCon) CreateVersion(c context.Context,
	p *entity.EriusScenario, author string, pipelineData []byte) error {
	c, span := trace.StartSpan(c, "pg_create_version")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	qNewVersion := `
	INSERT INTO versions (
		id, 
		status, 
		pipeline_id, 
		created_at, 
		content, 
		author, 
		comment,
		updated_at
	)
	VALUES (
		$1, 
		$2, 
		$3, 
		$4, 
		$5, 
		$6, 
		$7,
		$8
	)`

	createdAt := time.Now()

	_, err := db.Connection.Exec(c, qNewVersion, p.VersionID, StatusDraft, p.ID, createdAt, pipelineData, author, p.Comment, createdAt)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGCon) PipelineNameCreatable(c context.Context, name string) (bool, error) {
	c, span := trace.StartSpan(c, "pg_pipeline_name_creatable")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := `
	SELECT count(name) AS count
	FROM pipelines
	WHERE name = $1`

	row := db.Connection.QueryRow(c, q, name)

	count := 0

	err := row.Scan(&count)
	if err != nil {
		return false, err
	}

	if count != 0 {
		return false, nil
	}

	return true, nil
}

func (db *PGCon) CreateTag(c context.Context,
	e *entity.EriusTagInfo, author string) (*entity.EriusTagInfo, error) {
	c, span := trace.StartSpan(c, "pg_create_tag")
	defer span.End()

	tx, err := db.Connection.Begin(c)
	if err != nil {
		return nil, err
	}

	defer tx.Rollback(c) // nolint:errcheck // rollback err

	if e.Name == "" {
		return nil, nil
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
	FROM tags t
	WHERE 
		lower(t.name) = lower($1) AND t.status <> $2 and t.is_marker <> $3
	LIMIT 1`

	rows, err := tx.Query(c, qCheckTagExisted, e.Name, StatusDeleted, e.IsMarker)
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
		INSERT INTO tags (
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

	row := tx.QueryRow(c, qNewTag, e.ID, e.Name, StatusDraft, author, e.Color, e.IsMarker)

	etag := &entity.EriusTagInfo{}

	err = row.Scan(&etag.ID, &etag.Name, &etag.Status, &etag.Color, &etag.IsMarker)
	if err != nil {
		return nil, err
	}

	if commitErr := tx.Commit(c); commitErr != nil {
		return nil, commitErr
	}

	return etag, nil
}

func (db *PGCon) DeleteVersion(c context.Context, versionID uuid.UUID) error {
	c, span := trace.StartSpan(c, "pg_delete_version")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := `
		UPDATE versions 
		SET 
			deleted_at = $1, 
			status = $2 
		WHERE id = $3`
	t := time.Now()

	_, err := db.Connection.Exec(c, q, t, StatusDeleted, versionID)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGCon) deleteAllVersions(c context.Context, id uuid.UUID) error {
	c, span := trace.StartSpan(c, "pg_delete_all_versions")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := `
		UPDATE versions 
		SET 
			deleted_at = $1, 
			status = $2 
		WHERE pipeline_id = $3`
	t := time.Now()

	_, err := db.Connection.Exec(c, q, t, StatusDeleted, id)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGCon) DeletePipeline(c context.Context, id uuid.UUID) error {
	c, span := trace.StartSpan(c, "pg_delete_pipeline")
	defer span.End()

	tx, err := db.Connection.Begin(c)
	if err != nil {
		return err
	}
	defer tx.Rollback(c) // nolint:errcheck // rollback err

	t := time.Now()

	// nolint:gocritic
	// language=PostgreSQL
	qName := `
		SELECT name 
		FROM pipelines 
		WHERE id = $1`
	row := tx.QueryRow(c, qName, id)

	var n string

	err = row.Scan(&n)
	if err != nil {
		return err
	}

	n = n + "_deleted_at_" + t.String()

	// nolint:gocritic
	// language=PostgreSQL
	q := `
		UPDATE pipelines 
		SET 
			deleted_at = $1, 
			name = $2 
		WHERE id = $3`

	_, err = tx.Exec(c, q, t, n, id)
	if err != nil {
		return err
	}

	err = db.deleteAllVersions(c, id)
	if err != nil {
		return err
	}

	return tx.Commit(c)
}

func (db *PGCon) GetPipeline(c context.Context, id uuid.UUID) (*entity.EriusScenario, error) {
	c, span := trace.StartSpan(c, "pg_get_pipeline")
	defer span.End()

	p := entity.EriusScenario{}
	// nolint:gocritic
	// language=PostgreSQL
	q := `
	SELECT 
		pv.id, 
		pv.status, 
		pv.pipeline_id, 
		pv.content, 
		pv.comment,
		pv.author
	FROM versions pv
	JOIN pipeline_history pph ON pph.version_id = pv.id
	WHERE pv.pipeline_id = $1
	ORDER BY pph.date DESC 
	LIMIT 1
`

	rows, err := db.Connection.Query(c, q, id)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	if rows.Next() {
		var (
			vID, pID uuid.UUID
			s        int
			content  string
			cm       string
			author   string
		)

		err = rows.Scan(&vID, &s, &pID, &content, &cm, &author)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal([]byte(content), &p)
		if err != nil {
			return nil, err
		}

		p.VersionID = vID
		p.ID = pID
		p.Status = s
		p.Comment = cm

		if p.Author == "" {
			p.Author = author
		}

		return &p, nil
	}

	return nil, errCantFindPipelineVersion
}

func (db *PGCon) GetPipelineVersion(c context.Context, id uuid.UUID) (*entity.EriusScenario, error) {
	c, span := trace.StartSpan(c, "pg_get_pipeline_version")
	defer span.End()

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
	FROM versions pv
    LEFT JOIN pipeline_history pph ON pph.version_id = pv.id
	WHERE pv.id = $1
	ORDER BY pph.date DESC 
	LIMIT 1`

	rows, err := db.Connection.Query(c, qVersion, id)
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

func (db *PGCon) RenamePipeline(c context.Context, id uuid.UUID, name string) error {
	c, span := trace.StartSpan(c, "pg_rename_pipeline")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	const query = `
	WITH id_values (name) as (
      values ($1)
    ), src AS (
      UPDATE pipelines
          SET name = (select name from id_values)
      WHERE id = $2
    )
    UPDATE versions
       SET content = jsonb_set(content, '{name}', to_jsonb((select name from id_values)) , false)
    WHERE versions.id = 
          (SELECT ID 
           FROM versions ver 
           WHERE ver.pipeline_id = $2 ORDER BY created_at DESC LIMIT 1) 
    ;`

	_, err := db.Connection.Exec(c, query, name, id)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGCon) GetTag(c context.Context, e *entity.EriusTagInfo) (*entity.EriusTagInfo, error) {
	c, span := trace.StartSpan(c, "pg_get_tag")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	qGetTag := `
	SELECT 
		t.id, 
		t.name, 
		t.status, 
		t.color, 
		t.is_marker
	FROM tags t
	WHERE 
		t.id = $1 AND t.status <> $2
	LIMIT 1`

	rows, err := db.Connection.Query(c, qGetTag, e.ID, StatusDeleted)
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

func (db *PGCon) EditTag(c context.Context, e *entity.EriusTagInfo) error {
	c, span := trace.StartSpan(c, "pg_edit_tag")
	defer span.End()

	tx, err := db.Connection.Begin(c)
	if err != nil {
		return err
	}
	defer tx.Rollback(c) // nolint:errcheck // rollback err

	// nolint:gocritic
	// language=PostgreSQL
	qCheckTagIsCreated := `
		SELECT count(id) AS count
		FROM tags 
		WHERE id = $1 AND status = $2`

	row := tx.QueryRow(c, qCheckTagIsCreated, e.ID, StatusDraft)

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
	qEditTag := `UPDATE tags
	SET color = $1
	WHERE id = $2`

	_, err = tx.Exec(c, qEditTag, e.Color, e.ID)
	if err != nil {
		return err
	}

	if commitErr := tx.Commit(c); commitErr != nil {
		return commitErr
	}

	return nil
}

//nolint:dupl //its different
func (db *PGCon) AttachTag(c context.Context, pid uuid.UUID, e *entity.EriusTagInfo) error {
	c, span := trace.StartSpan(c, "pg_attach_tag")
	defer span.End()

	tx, err := db.Connection.Begin(c)
	if err != nil {
		return err
	}

	defer tx.Rollback(c) // nolint:errcheck // rollback err

	row := tx.QueryRow(c, qCheckTagIsAttached, pid, e.ID)

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
	INSERT INTO pipeline_tags (
		pipeline_id, 
		tag_id
	)
	VALUES (
		$1, 
		$2
	)`

	_, err = tx.Exec(c, qAttachTag, pid, e.ID)
	if err != nil {
		return err
	}

	if commitErr := tx.Commit(c); commitErr != nil {
		return commitErr
	}

	return nil
}

func (db *PGCon) RemoveTag(c context.Context, id uuid.UUID) error {
	c, span := trace.StartSpan(c, "pg_remove_tag")
	defer span.End()

	tx, err := db.Connection.Begin(c)
	if err != nil {
		return err
	}
	defer tx.Rollback(c) // nolint:errcheck // rollback err

	t := time.Now()

	// nolint:gocritic
	// language=PostgreSQL
	qName := `SELECT 
		name 
	FROM tags 
	WHERE id = $1`

	row := tx.QueryRow(c, qName, id)

	var n string

	err = row.Scan(&n)
	if err != nil {
		return err
	}

	n = n + "_deleted_at_" + t.String()

	// nolint:gocritic
	// language=PostgreSQL
	qSetTagDeleted := `UPDATE tags
	SET 
		status = $1, 
		name = $2  
	WHERE id = $3`

	_, err = tx.Exec(c, qSetTagDeleted, StatusDeleted, n, id)
	if err != nil {
		return err
	}

	// nolint:gocritic
	// language=PostgreSQL
	qCheckTagAttached := `
	SELECT COUNT(pipeline_id) AS count
	FROM pipeline_tags
	WHERE tag_id = $1`

	row = tx.QueryRow(c, qCheckTagAttached, id)

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
	DELETE FROM pipeline_tags
	WHERE tag_id = $1`

	_, err = tx.Exec(c, qRemoveAttachedTags, id)
	if err != nil {
		return err
	}

	return tx.Commit(c)
}

//nolint:dupl //its different
func (db *PGCon) DetachTag(c context.Context, pid uuid.UUID, e *entity.EriusTagInfo) error {
	c, span := trace.StartSpan(c, "pg_detach_tag")
	defer span.End()

	tx, err := db.Connection.Begin(c)
	if err != nil {
		return err
	}

	defer tx.Rollback(c) // nolint:errcheck // rollback err

	row := tx.QueryRow(c, qCheckTagIsAttached, pid, e.ID)

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
	qDetachTag := `DELETE FROM pipeline_tags
	WHERE pipeline_id = $1 
	AND tag_id = $2`

	_, err = tx.Exec(c, qDetachTag, pid, e.ID)
	if err != nil {
		return err
	}

	if commitErr := tx.Commit(c); commitErr != nil {
		return commitErr
	}

	return nil
}

func (db *PGCon) RemovePipelineTags(c context.Context, id uuid.UUID) error {
	c, span := trace.StartSpan(c, "pg_remove_pipeline_tags")
	defer span.End()

	tx, err := db.Connection.Begin(c)
	if err != nil {
		return err
	}

	defer tx.Rollback(c) // nolint:errcheck // rollback err

	// nolint:gocritic
	// language=PostgreSQL
	qCheckTagIsAttached := `
	SELECT COUNT(pipeline_id) AS count
	FROM pipeline_tags
	WHERE pipeline_id = $1`

	row := tx.QueryRow(c, qCheckTagIsAttached, id)

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
	DELETE FROM pipeline_tags
	WHERE pipeline_id = $1`

	_, err = tx.Exec(c, qRemovePipelineTags, id)
	if err != nil {
		return err
	}

	if commitErr := tx.Commit(c); commitErr != nil {
		return commitErr
	}

	return nil
}

func (db *PGCon) UpdateDraft(c context.Context,
	p *entity.EriusScenario, pipelineData []byte) error {
	c, span := trace.StartSpan(c, "pg_update_draft")
	defer span.End()

	tx, err := db.Connection.Begin(c)
	if err != nil {
		return err
	}
	defer tx.Rollback(c) // nolint:errcheck // rollback err

	// nolint:gocritic
	// language=PostgreSQL
	q := `
	UPDATE versions 
	SET 
		status = $1, 
		content = $2, 
		comment = $3,
		is_actual = $4,
		updated_at = $5
	WHERE id = $6`

	_, err = tx.Exec(c, q, p.Status, pipelineData, p.Comment, p.Status == StatusApproved, time.Now(), p.VersionID)
	if err != nil {
		return err
	}

	if p.Status == StatusApproved {
		q = `
	UPDATE versions
	SET is_actual = FALSE
	WHERE id != $1
	AND pipeline_id = $2`
		_, err = tx.Exec(c, q, p.VersionID, p.ID)
		if err != nil {
			return err
		}
	}

	return tx.Commit(c)
}

func (db *PGCon) SaveStepContext(ctx context.Context, dto *SaveStepRequest) (uuid.UUID, time.Time, error) {
	ctx, span := trace.StartSpan(ctx, "pg_save_step_context")
	defer span.End()

	var id uuid.UUID
	var t time.Time

	const q = `
		SELECT id, time
			FROM variable_storage 
		WHERE work_id = $1 AND
			step_name = $2 AND
			status IN ('idle', 'ready', 'running')
`

	if scanErr := db.Connection.QueryRow(ctx, q, dto.WorkID, dto.StepName).
		Scan(&id, &t); scanErr != nil && !errors.Is(scanErr, pgx.ErrNoRows) {
		return NullUuid, time.Time{}, nil
	}

	if id != NullUuid {
		return id, t, nil
	}
	id = uuid.New()
	timestamp := time.Now()
	// nolint:gocritic
	// language=PostgreSQL
	const query = `
		INSERT INTO variable_storage (
			id, 
			work_id, 
			step_type,
			step_name, 
			content, 
			time, 
			break_points, 
			has_error,
			status,
		    check_sla,
		    sla_deadline,
		    check_half_sla
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
			$9,
			$10,
		    $11,
			$12
		)
`

	_, err := db.Connection.Exec(
		ctx,
		query,
		id,
		dto.WorkID,
		dto.StepType,
		dto.StepName,
		dto.Content,
		timestamp,
		dto.BreakPoints,
		dto.HasError,
		dto.Status,
		dto.CheckSLA,
		dto.SLADeadline,
		dto.CheckHalfSLA,
	)
	if err != nil {
		return NullUuid, time.Time{}, err
	}

	err = db.insertIntoMembers(ctx, dto.Members, id)
	if err != nil {
		return NullUuid, time.Time{}, err
	}

	return id, timestamp, nil
}

func (db *PGCon) UpdateStepContext(ctx context.Context, dto *UpdateStepRequest) error {
	c, span := trace.StartSpan(ctx, "pg_update_step_context")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := `
	UPDATE variable_storage
	SET
		break_points = $2
		, has_error = $3
		, status = $4
		, check_sla = $5
		, content = $6
		, updated_at = NOW()
		, sla_deadline = $7
		, check_half_sla = $8
	WHERE
		id = $1
`
	args := []interface{}{dto.Id, dto.BreakPoints, dto.HasError, dto.Status, dto.CheckSLA, dto.Content, dto.SLADeadline, dto.CheckHalfSLA}

	_, err := db.Connection.Exec(
		c,
		q,
		args...,
	)
	if err != nil {
		return err
	}

	// nolint:gocritic
	// language=PostgreSQL
	const qMembersDelete = `
		DELETE FROM members 
		WHERE block_id = $1
`
	_, err = db.Connection.Exec(
		ctx,
		qMembersDelete,
		dto.Id,
	)
	if err != nil {
		return err
	}

	err = db.insertIntoMembers(ctx, dto.Members, dto.Id)
	if err != nil {
		return err
	}
	return nil
}

func (db *PGCon) insertIntoMembers(ctx context.Context, members []DbMember, id uuid.UUID) error {

	// nolint:gocritic
	// language=PostgreSQL
	const queryMembers = `
		INSERT INTO members (               
			id,
		     block_id,
		    login,
		    finished,
		     actions                
		)
		VALUES (
			$1, 
			$2, 
			$3, 
			$4, 
			$5
		)
`
	for _, val := range members {
		membersId := uuid.New()
		actions := make(pq.StringArray, 0, len(val.Actions))
		for _, act := range val.Actions {
			actions = append(actions, act.Id+":"+act.Type)
		}
		_, err := db.Connection.Exec(
			ctx,
			queryMembers,
			membersId,
			id,
			val.Login,
			val.Finished,
			actions,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (db *PGCon) GetExecutableScenarios(c context.Context) ([]entity.EriusScenario, error) {
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
	FROM versions pv
	JOIN pipelines pp ON pv.pipeline_id = pp.id
	JOIN pipeline_history ph ON ph.version_id = pv.id
	WHERE 
		pv.status = $1
		AND pp.deleted_at is NULL
	ORDER BY pv.created_at`

	rows, err := db.Connection.Query(c, q, StatusApproved)
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

func (db *PGCon) GetExecutableByName(c context.Context, name string) (*entity.EriusScenario, error) {
	c, span := trace.StartSpan(c, "pg_get_executable_by_name")
	defer span.End()

	p := entity.EriusScenario{}
	// nolint:gocritic
	// language=PostgreSQL
	q := `
	SELECT 
		pv.id, 
		pv.status, 
		pv.pipeline_id, 
		pv.content
	FROM versions pv
	JOIN pipeline_history pph on pph.version_id = pv.id
	JOIN pipelines p on p.id = pv.pipeline_id
	WHERE 
		p.name = $1 
		AND p.deleted_at IS NULL
	ORDER BY pph.date DESC 
	LIMIT 1
`

	rows, err := db.Connection.Query(c, q, name)
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

func (db *PGCon) GetUnfinishedTaskStepsByWorkIdAndStepType(ctx context.Context, id uuid.UUID, stepType string,
) (entity.TaskSteps, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_unfinished_task_steps_by_work_id_and_step_type")
	defer span.End()

	el := entity.TaskSteps{}

	var notInStatuses []string
	if stepType == "form" {
		notInStatuses = []string{"skipped"}
	} else {
		notInStatuses = []string{"skipped", "finished"}
	}

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
	FROM variable_storage vs 
	WHERE 
	    work_id = $1 AND 
	    step_type = $2
	    AND NOT status = ANY($3)
	    ORDER BY vs.time ASC`

	rows, err := db.Connection.Query(ctx, q, id, stepType, notInStatuses)
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

func (db *PGCon) GetTaskStepsToWait(ctx context.Context, workNumber, blockName string) ([]string, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_task_steps_to_wait")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := `WITH blocks AS (
    SELECT key(JSONB_EACH(content -> 'pipeline' -> 'blocks'))                                as key,
           value(jsonb_each(value(jsonb_each(content -> 'pipeline' -> 'blocks')) -> 'next')) as value
    FROM versions v
    WHERE v.id = (SELECT version_id FROM works WHERE work_number = $1))
SELECT DISTINCT key
FROM blocks
WHERE value ? $2`

	var blocks []string
	rows, err := db.Connection.Query(ctx, q, workNumber, blockName)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var b string
		if scanErr := rows.Scan(&b); scanErr != nil {
			return nil, scanErr
		}
		blocks = append(blocks, b)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}
	return blocks, nil
}

func (db *PGCon) CheckTaskStepsExecuted(ctx context.Context, workNumber string, blocks []string) (bool, error) {
	ctx, span := trace.StartSpan(ctx, "pg_check_task_steps_executed")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := `
	SELECT count(*)
	FROM variable_storage vs 
	WHERE vs.work_id = (
	    SELECT id FROM works WHERE work_number = $1
	) AND vs.step_name = ANY($2) AND vs.status IN ('finished', 'no_success')`
	// TODO: rewrite to handle edits ?

	var c int
	if scanErr := db.Connection.QueryRow(ctx, q, workNumber, blocks).Scan(&c); scanErr != nil {
		return false, nil
	}
	return c == len(blocks), nil
}

func (db *PGCon) GetTaskStepById(ctx context.Context, id uuid.UUID) (*entity.Step, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_task_step_by_id")
	defer span.End()

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
	FROM variable_storage vs 
	WHERE id = $1
	LIMIT 1`

	var s entity.Step
	var content string
	err := db.Connection.QueryRow(ctx, q, id).Scan(
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

//nolint:dupl //its not duplicate
func (db *PGCon) GetParentTaskStepByName(ctx context.Context,
	workID uuid.UUID, stepName string) (*entity.Step, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_parent_task_step_by_name")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	const query = `
		SELECT 
			vs.id,
			vs.step_type,
			vs.step_name, 
			vs.time, 
			vs.content, 
			COALESCE(vs.break_points, '{}') AS break_points, 
			vs.has_error,
			vs.status
		FROM variable_storage vs 
			LEFT JOIN works w ON w.child_id = $1 
		WHERE vs.work_id = w.id AND vs.step_name = $2
		LIMIT 1
`

	var s entity.Step
	var content string
	err := db.Connection.QueryRow(ctx, query, workID, stepName).Scan(
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

	if err = json.Unmarshal([]byte(content), storage); err != nil {
		return nil, err
	}

	s.State = storage.State
	s.Steps = storage.Steps
	s.Errors = storage.Errors
	s.Storage = storage.Values

	return &s, nil
}

//nolint:dupl //its not duplicate
func (db *PGCon) GetTaskStepByName(ctx context.Context, workID uuid.UUID, stepName string) (*entity.Step, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_task_step_by_name")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	const query = `
		SELECT 
			vs.id,
			vs.step_type,
			vs.step_name, 
			vs.time, 
			vs.content, 
			COALESCE(vs.break_points, '{}') AS break_points, 
			vs.has_error,
			vs.status
		FROM variable_storage vs  
			WHERE vs.work_id = $1 AND vs.step_name = $2
		LIMIT 1
`

	var s entity.Step
	var content string
	err := db.Connection.QueryRow(ctx, query, workID, stepName).Scan(
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

func (db *PGCon) getVersionHistory(c context.Context, id uuid.UUID, status int) ([]entity.EriusVersionInfo, error) {
	c, span := trace.StartSpan(c, "pg_get_version_history")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := `
	SELECT 
		pv.id,
	    pv.approver,
	    pv.author, 
		pv.created_at, 
	    pv.updated_at,
		pv.is_actual,
	    pv.status
	FROM versions pv
	JOIN pipelines pp ON pv.pipeline_id = pp.id
	WHERE 
		pp.id = $1 
		--status--
		AND pp.deleted_at IS NULL
	ORDER BY created_at DESC`

	if status != -1 {
		q = strings.Replace(q, "--status--", fmt.Sprintf("AND pv.status=%d", status), 1)
	}

	rows, err := db.Connection.Query(c, q, id)
	if err != nil {
		return nil, err
	}

	res, err := parseRowsVersionHistoryList(c, rows)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (db *PGCon) GetVersionByWorkNumber(c context.Context, workNumber string) (*entity.EriusScenario, error) {
	c, span := trace.StartSpan(c, "pg_get_version_by_work_number")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	const query = `
		SELECT
			version.id,
			version.status,
			version.pipeline_id,
			version.created_at,
			version.content,
			version.comment_rejected,
			version.comment,
			version.author,
			(SELECT MAX(date) FROM pipeline_history 
				WHERE pipeline_id = version.pipeline_id
			) AS last_approve
		FROM works work
			LEFT JOIN versions version ON version.id = work.version_id
		WHERE work.work_number = $1 AND work.child_id IS NULL;
`

	row := db.Connection.QueryRow(c, query, workNumber)

	var (
		vID, pID uuid.UUID
		s        int
		content  string
		cr       string
		cm       string
		d        *time.Time
		ca       *time.Time
		a        string
	)

	err := row.Scan(&vID, &s, &pID, &ca, &content, &cr, &cm, &a, &d)
	if err != nil {
		return nil, err
	}

	res := &entity.EriusScenario{}

	err = json.Unmarshal([]byte(content), &res)
	if err != nil {
		return nil, err
	}

	res.VersionID = vID
	res.ID = pID
	res.Status = s
	res.CommentRejected = cr
	res.Comment = cm
	res.ApprovedAt = d
	res.CreatedAt = ca
	res.Author = a

	return res, nil
}

func (db *PGCon) GetVersionsByPipelineID(c context.Context, pID string) ([]entity.EriusScenario, error) {
	c, span := trace.StartSpan(c, "pg_get_versions_by_blueprint_id")
	defer span.End()

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
		(SELECT MAX(date) FROM pipeline_history WHERE pipeline_id = pv.pipeline_id) AS last_approve
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
										FROM versions
									) as pipeline
						   ) as next_from_start
					  WHERE next_from_start.nextNode LIKE 'servicedesk_application%'
				  ) as servicedesk_node
	) as servicedesk_node_params
		LEFT JOIN versions pv ON pv.id = servicedesk_node_params.pipeline_version_id
	WHERE pv.status = 2 AND
			pv.is_actual = TRUE AND
			pv.pipeline_id = $1 AND
			servicedesk_node_params.type_id = 'servicedesk_application';
`

	rows, err := db.Connection.Query(c, query, pID)
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

func (db *PGCon) GetPipelinesByNameOrId(ctx context.Context, dto *SearchPipelineRequest) ([]entity.SearchPipeline, error) {
	c, span := trace.StartSpan(ctx, "pg_get_pipelines_by_name_or_id")
	defer span.End()

	res := make([]entity.SearchPipeline, 0, dto.Limit)

	// nolint:gocritic
	// language=PostgreSQL
	var q = `
		SELECT
			p.id,
			p.name,
			count(*) over () as total
		FROM pipelines p
		WHERE p.deleted_at IS NULL  --pipe--
		LIMIT $1 OFFSET $2;
`

	if dto.PipelineName != nil {
		q = strings.ReplaceAll(q, "--pipe--", fmt.Sprintf("AND p.name ilike'%%%s%%'", *dto.PipelineName))
	}

	if dto.PipelineId != nil {
		q = strings.ReplaceAll(q, "--pipe--", fmt.Sprintf("AND p.id='%s'", *dto.PipelineId))
	}

	rows, err := db.Connection.Query(c, q, dto.Limit, dto.Offset)
	if err != nil {
		return res, err
	}

	defer rows.Close()

	//nolint:dupl //scan
	for rows.Next() {
		s := entity.SearchPipeline{}
		err = rows.Scan(
			&s.PipelineId,
			&s.PipelineName,
			&s.Total,
		)
		if err != nil {
			return res, err
		}

		res = append(res, s)
	}

	return res, nil
}

func (db *PGCon) CheckUserCanEditForm(ctx context.Context, workNumber, stepName, login string) (bool, error) {
	ctx, span := trace.StartSpan(ctx, "check_user_can_edit_form")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	var q = `
			with accesses as (
				select jsonb_array_elements(content -> 'State' -> step_name -> 'forms_accessibility') as data
				from variable_storage
				where step_type = 'approver'
				  and content -> 'State' -> step_name -> 'approvers' ? $3
				  and work_id = (SELECT id
								 FROM works
								 WHERE work_number = $1
				      )
			union
			select jsonb_array_elements(content -> 'State' -> step_name -> 'forms_accessibility') as data
			from variable_storage
			where step_type = 'execution'
			  and content -> 'State' -> step_name -> 'executors' ? $3
			  and work_id = (SELECT id
							 FROM works
							 WHERE work_number = $1))
			select count(*) from accesses
			where accesses.data::jsonb ->> 'node_id' = $2 and accesses.data::jsonb ->> 'accessType' = 'ReadWrite'
`
	var count int
	if scanErr := db.Connection.QueryRow(ctx, q, workNumber, stepName, login).Scan(&count); scanErr != nil {
		return false, scanErr
	}

	return count != 0, nil
}

func (db *PGCon) GetTaskRunContext(ctx context.Context, workNumber string) (entity.TaskRunContext, error) {
	ctx, span := trace.StartSpan(ctx, "get_task_run_context")
	defer span.End()

	var runCtx entity.TaskRunContext

	// language=PostgreSQL
	q := `
		SELECT run_context
		FROM works
		WHERE work_number = $1`

	if scanErr := db.Connection.QueryRow(ctx, q, workNumber).Scan(&runCtx); scanErr != nil {
		return runCtx, scanErr
	}
	return runCtx, nil
}

func (db *PGCon) GetBlockDataFromVersion(ctx context.Context, workNumber, blockName string) (*entity.EriusFunc, error) {
	ctx, span := trace.StartSpan(ctx, "get_block_data_from_version")
	defer span.End()

	q := `
		SELECT content->'pipeline'->'blocks'->$1 FROM versions
    	JOIN works w ON versions.id = w.version_id
		WHERE w.work_number = $2`

	var f *entity.EriusFunc

	if scanErr := db.Connection.QueryRow(ctx, q, blockName, workNumber).Scan(&f); scanErr != nil {
		return nil, scanErr
	}
	return f, nil
}

func (db *PGCon) StopTaskBlocks(ctx context.Context, taskID uuid.UUID) error {
	ctx, span := trace.StartSpan(ctx, "stop_task_blocks")
	defer span.End()

	q := `
		UPDATE variable_storage
		SET status = 'cancel'
		WHERE work_id = $1 AND status IN ('ready', 'idle', 'running')`

	_, err := db.Connection.Exec(ctx, q, taskID)
	return err
}

func (db *PGCon) GetVariableStorageForStep(ctx context.Context, taskID uuid.UUID, stepType string) (*store.VariableStore, error) {
	ctx, span := trace.StartSpan(ctx, "stop_task_blocks")
	defer span.End()

	q := `
		SELECT content
		FROM variable_storage
		WHERE work_id = $1 AND step_name = $2`

	var content []byte
	if err := db.Connection.QueryRow(ctx, q, taskID, stepType).Scan(&content); err != nil {
		return nil, err
	}
	storage := store.NewStore()
	if err := json.Unmarshal(content, &storage); err != nil {
		return nil, err
	}
	return storage, nil
}

func (db *PGCon) GetBlocksBreachedSLA(ctx context.Context) ([]StepBreachedSLA, error) {
	ctx, span := trace.StartSpan(ctx, "get_blocks_breached_sla")
	defer span.End()

	// language=PostgreSQL
	// half_sla = (vs.time + (vs.sla_deadline - vs.time)/2)
	q := `
		SELECT w.id,
		       w.work_number,
		       p.name,	
		       v.author,
		       vs.content,
		       v.content->'pipeline'->'blocks'->vs.step_name,
		       vs.step_name,
		       (case when sla_deadline > NOW() THEN False ELSE True END) already
		FROM variable_storage vs 
		    JOIN works w on vs.work_id = w.id 
		    JOIN versions v on w.version_id = v.id
			JOIN pipelines p on v.pipeline_id = p.id
		WHERE (
		    (check_sla = True AND sla_deadline < NOW()) or 
		    (vs.check_half_sla = True AND (vs.time + (vs.sla_deadline - vs.time)/2) < NOW())
		) 
		  AND vs.status = 'running'`
	rows, err := db.Connection.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := make([]StepBreachedSLA, 0)
	for rows.Next() {
		var content []byte
		item := StepBreachedSLA{}
		if scanErr := rows.Scan(
			&item.TaskID,
			&item.WorkNumber,
			&item.WorkTitle,
			&item.Initiator,
			&content,
			&item.BlockData,
			&item.StepName,
			&item.Already,
		); scanErr != nil {
			return nil, scanErr
		}
		storage := store.NewStore()
		if unmErr := json.Unmarshal(content, &storage); unmErr != nil {
			return nil, unmErr
		}
		item.VarStore = storage

		res = append(res, item)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}
	return res, nil
}
