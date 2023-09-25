package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"go.opencensus.io/trace"

	"golang.org/x/exp/slices"

	"github.com/google/uuid"

	"github.com/lib/pq"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/configs"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type PGCon struct {
	Connection Connector
}

var (
	NullUuid = [16]byte{}
)

type Connector interface {
	Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, optionsAndArgs ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, optionsAndArgs ...interface{}) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

func (db *PGCon) Ping(ctx context.Context) error {
	if pingConn, ok := db.Connection.(interface {
		Ping(ctx context.Context) error
	}); ok {
		return pingConn.Ping(ctx)
	}
	return errors.New("can't ping dn")
}

func (db *PGCon) StartTransaction(ctx context.Context) (Database, error) {
	_, span := trace.StartSpan(ctx, "start_transaction")
	defer span.End()

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
	_, span := trace.StartSpan(ctx, "rollback_transaction")
	defer span.End()

	tx, ok := db.Connection.(pgx.Tx)
	if !ok {
		return nil
	}
	return tx.Rollback(ctx) // nolint:errcheck // rollback err
}

func ConnectPostgres(ctx context.Context, db *configs.Database) (PGCon, error) {
	maxConnections := strconv.Itoa(db.MaxConnections)
	connString := "postgres://" + os.Getenv(db.UserEnvKey) + ":" + os.Getenv(db.PassEnvKey) +
		"@" + db.Host + ":" + db.Port + "/" + db.DBName +
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
	RunStatusCanceled int = 6

	CommentCanceled = "Заявка отозвана администратором платформы автоматизации"

	SystemLogin = "jocasta"

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

	startSchema   = "start_schema"
	endSchema     = "end_schema"
	inputSchema   = "input_schema"
	outputSchema  = "output_schema"
	inputMapping  = "input_mapping"
	outputMapping = "output_mapping"
)

var (
	errCantFindPipelineVersion = errors.New("can't find pipeline version")
	errCantFindTag             = errors.New("can't find tag")
	errCantFindExternalSystem  = errors.New("can't find external system settings")
	errUnkonwnSchemaFlag       = errors.New("unknown schema flag")
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

func (db *PGCon) GetPipelinesWithLatestVersion(c context.Context,
	authorLogin string,
	publishedPipelines bool,
	page, perPage *int,
	filter string) ([]entity.EriusScenarioInfo, error) {
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
      AND pv2.status NOT IN ---versions_status---
)
  ---author---
`

	if authorLogin != "" {
		q = strings.ReplaceAll(q, "---author---", "AND pv.author='"+authorLogin+"'")
	}

	if filter != "" {
		escapeFilter := strings.Replace(filter, "_", "!_", -1)
		escapeFilter = strings.Replace(escapeFilter, "%", "!%", -1)
		q = fmt.Sprintf(`%s AND (pp.name ILIKE '%%%s%%' ESCAPE '!')`, q, escapeFilter)
	}

	if publishedPipelines {
		q = strings.ReplaceAll(q, "---versions_status---", "(1, 3)")
	} else {
		q = strings.ReplaceAll(q, "---versions_status---", "(3)")
	}

	q = fmt.Sprintf("%s ORDER BY created_at", q)

	if page != nil && perPage != nil {
		q = fmt.Sprintf("%s OFFSET %d", q, *page**perPage)
	}

	if perPage != nil {
		q = fmt.Sprintf("%s LIMIT %d", q, *perPage)
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

	return db.CreateVersion(c, p, author, pipelineData, uuid.Nil)
}

func (db *PGCon) CreateVersion(c context.Context,
	p *entity.EriusScenario, author string, pipelineData []byte, oldVersionID uuid.UUID) error {
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

	if oldVersionID != uuid.Nil {
		err = db.copyProcessSettingsFromOldVersion(c, p.VersionID, oldVersionID)
		if err != nil {
			return err
		}
	} else {
		err = db.SaveVersionSettings(c, entity.ProcessSettings{Id: p.VersionID.String(), ResubmissionPeriod: 0}, nil)
		if err != nil {
			return err
		}
		err = db.SaveSlaVersionSettings(c, p.VersionID.String(), entity.SlaVersionSettings{
			Author:   author,
			WorkType: "8/5",
			Sla:      40,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (db *PGCon) copyProcessSettingsFromOldVersion(c context.Context, newVersionID, oldVersionID uuid.UUID) error {
	qCopyPrevSettings := `
	INSERT INTO version_settings (id, version_id, start_schema, end_schema, resubmission_period) 
		SELECT uuid_generate_v4(), $1, start_schema, end_schema, resubmission_period
		FROM version_settings 
		WHERE version_id = $2
	`

	_, err := db.Connection.Exec(c, qCopyPrevSettings, newVersionID, oldVersionID)
	if err != nil {
		return err
	}

	qCopyExternalSystems := `
	INSERT INTO external_systems (id, version_id, system_id, input_schema, output_schema, input_mapping, output_mapping,
                              microservice_id, ending_url, sending_method, allow_run_as_others)
SELECT uuid_generate_v4(),
       $1,
       system_id,
       input_schema,
       output_schema,
       input_mapping,
       output_mapping,
       microservice_id,
       ending_url,
       sending_method,
       allow_run_as_others
FROM external_systems
WHERE version_id = $2;
	`

	_, err = db.Connection.Exec(c, qCopyExternalSystems, newVersionID, oldVersionID)
	if err != nil {
		return err
	}

	// nolint:gocritic
	// language=PostgreSQL
	qCopyPrevSlaSettings := `
	INSERT INTO version_sla (id, version_id, author,created_at,work_type,sla) 
		SELECT uuid_generate_v4(), $1, author, now(), work_type, sla
		FROM version_sla 
		WHERE version_id = $2
		ORDER BY created_at DESC LIMIT 1;
	`

	_, err = db.Connection.Exec(c, qCopyPrevSlaSettings, newVersionID, oldVersionID)
	if err != nil {
		return err
	}

	// nolint:gocritic
	// language=PostgreSQL
	qCopyPrevTaskSubSettings := `
INSERT INTO external_system_task_subscriptions (id, version_id, system_id, microservice_id, path, 
                                                method, notification_schema, mapping, nodes)
SELECT uuid_generate_v4(), $1, system_id, microservice_id, path, method, notification_schema, mapping, nodes 
FROM external_system_task_subscriptions
WHERE version_id = $2`

	_, err = db.Connection.Exec(c, qCopyPrevTaskSubSettings, newVersionID, oldVersionID)
	if err != nil {
		return err
	}

	return nil
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
	JOIN pipelines p ON p.id = pv.pipeline_id
	LEFT JOIN pipeline_history pph ON pph.version_id = pv.id
	WHERE pv.pipeline_id = $1 AND p.deleted_at IS NULL
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

func (db *PGCon) GetPipelineVersion(c context.Context, id uuid.UUID, checkNotDeleted bool) (*entity.EriusScenario, error) {
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
		vs.start_schema,
		vs.end_schema,
		pph.date
	FROM versions pv
	JOIN pipelines p ON pv.pipeline_id = p.id
    LEFT JOIN pipeline_history pph ON pph.version_id = pv.id
	LEFT JOIN version_settings vs on pv.id = vs.version_id
	WHERE pv.id = $1 --is_deleted--
	ORDER BY pph.date DESC 
	LIMIT 1`

	if checkNotDeleted {
		qVersion = strings.Replace(qVersion, "--is_deleted--", "AND p.deleted_at IS NULL", 1)
	}

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
			ss, es   *script.JSONSchema
			a        string
		)

		err := rows.Scan(&vID, &s, &pID, &ca, &c, &cr, &cm, &a, &ss, &es, &d)
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
		p.Settings.StartSchema = ss
		p.Settings.EndSchema = es
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
	p *entity.EriusScenario, pipelineData []byte, groups []*entity.NodeGroup) error {
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
		updated_at = $5,
		node_groups = $6
	WHERE id = $7`

	_, err = tx.Exec(c, q, p.Status, pipelineData, p.Comment, p.Status == StatusApproved, time.Now(), groups, p.VersionID)
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

	if !dto.IsReEntry {
		var id uuid.UUID
		var t time.Time
		const q = `
		SELECT id, time
			FROM variable_storage
			WHERE work_id = $1 AND
                step_name = $2 AND
                (status IN ('idle', 'ready', 'running') OR
					(
						step_type = 'form' AND
						status IN ('idle', 'ready', 'running', 'finished') AND
			    		time = (SELECT max(time) FROM variable_storage vs WHERE vs.work_id = $1 AND step_name = $2)
					)
			    )
	`

		if scanErr := db.Connection.QueryRow(ctx, q, dto.WorkID, dto.StepName).
			Scan(&id, &t); scanErr != nil && !errors.Is(scanErr, pgx.ErrNoRows) {
			return NullUuid, time.Time{}, scanErr
		}

		if id != NullUuid {
			return id, t, nil
		}
	}

	id := uuid.New()
	timestamp := time.Now()
	// nolint:gocritic
	// language=PostgreSQL
	query := `
		INSERT INTO variable_storage (
			id, 
			work_id, 
			step_type,
			step_name, 
			content, 
			time, 
			break_points, 
			has_error,
			status
			--update_col--
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
			--update_val--
		)
`
	args := []interface{}{
		id,
		dto.WorkID,
		dto.StepType,
		dto.StepName,
		dto.Content,
		timestamp,
		dto.BreakPoints,
		dto.HasError,
		dto.Status,
	}

	if _, ok := map[string]struct{}{
		"finished": {}, "no_success": {}, "error": {},
	}[dto.Status]; ok {
		args = append(args, timestamp)
		query = strings.Replace(query, "--update_col--", ",updated_at", 1)
		query = strings.Replace(query, "--update_val--", fmt.Sprintf(",$%d", len(args)), 1)
	}

	_, err := db.Connection.Exec(
		ctx,
		query,
		args...,
	)
	if err != nil {
		return NullUuid, time.Time{}, err
	}

	err = db.insertIntoMembers(ctx, dto.Members, id)
	if err != nil {
		return NullUuid, time.Time{}, err
	}

	err = db.deleteAndInsertIntoDeadlines(ctx, dto.Deadlines, id)
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
		, content = $5
		, updated_at = NOW()
	WHERE
		id = $1
`
	args := []interface{}{
		dto.Id, dto.BreakPoints, dto.HasError, dto.Status, dto.Content,
	}

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

	err = db.deleteAndInsertIntoDeadlines(ctx, dto.Deadlines, dto.Id)
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
			actions,
		    params                 
		)
		VALUES (
			$1, 
			$2, 
			$3, 
			$4, 
			$5,
		    $6
		)
`
	for _, val := range members {
		membersId := uuid.New()
		actions := make(pq.StringArray, 0, len(val.Actions))
		params := make(map[string]map[string]interface{})
		for _, act := range val.Actions {
			actions = append(actions, act.Id+":"+act.Type)
			if len(act.Params) != 0 {
				params[act.Id] = act.Params
			}
		}
		paramsData, mErr := json.Marshal(params)
		if mErr != nil {
			return mErr
		}
		_, err := db.Connection.Exec(
			ctx,
			queryMembers,
			membersId,
			id,
			val.Login,
			val.Finished,
			actions,
			paramsData,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (db *PGCon) insertIntoDeadlines(ctx context.Context, deadlines []DbDeadline, id uuid.UUID) error {
	// nolint:gocritic
	// language=PostgreSQL
	const queryDeadlines = `
		INSERT INTO deadlines(
			id,
			block_id,
			deadline,
			action
		)
		VALUES (
			$1, 
			$2, 
			$3,
		    $4
		)
`
	for _, val := range deadlines {
		deadlineId := uuid.New()
		_, err := db.Connection.Exec(
			ctx,
			queryDeadlines,
			deadlineId,
			id,
			val.Deadline,
			val.Action,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (db *PGCon) deleteDeadlines(ctx context.Context, id uuid.UUID) error {
	// nolint:gocritic
	// language=PostgreSQL
	const queryDeadlines = `
		DELETE from deadlines where block_id = $1
`
	_, err := db.Connection.Exec(
		ctx,
		queryDeadlines,
		id,
	)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGCon) deleteAndInsertIntoDeadlines(ctx context.Context, deadlines []DbDeadline, id uuid.UUID) error {
	deleteErr := db.deleteDeadlines(ctx, id)
	if deleteErr != nil {
		return deleteErr
	}

	insertErr := db.insertIntoDeadlines(ctx, deadlines, id)
	if insertErr != nil {
		return insertErr
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
		AND pv.deleted_at IS NULL
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
	action entity.TaskUpdateAction) (entity.TaskSteps, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_unfinished_task_steps_by_work_id_and_step_type")
	defer span.End()

	el := entity.TaskSteps{}

	var notInStatuses []string

	isAddInfoReq := slices.Contains([]entity.TaskUpdateAction{
		entity.TaskUpdateActionRequestApproveInfo,
		entity.TaskUpdateActionSLABreachRequestAddInfo,
		entity.TaskUpdateActionDayBeforeSLARequestAddInfo}, action)

	// nolint:gocritic,goconst
	if stepType == "form" {
		notInStatuses = []string{"skipped"}
	} else if (stepType == "execution" || stepType == "approver") && isAddInfoReq {
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
	    step_type = $2 AND 
	    NOT status = ANY($3) AND 
	    vs.time = (SELECT max(time) FROM variable_storage WHERE work_id = $1 AND step_name = vs.step_name)
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
    WHERE v.id = (SELECT version_id FROM works WHERE work_number = $1 AND child_id IS NULL))
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

func (db *PGCon) ParallelIsFinished(ctx context.Context, workNumber, blockName string) (bool, error) {
	ctx, span := trace.StartSpan(ctx, "pg_parallel_is_finished")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	const q = `with recursive all_nodes as(
		select distinct key(jsonb_each(v.content #> '{pipeline,blocks}'))::text out_node,
			jsonb_array_elements_text(value(jsonb_each(value(jsonb_each(v.content #> '{pipeline,blocks}'))->'next'))) as in_node
		from works w
		inner join versions v on w.version_id=v.id
		where w.work_number=$1
	),
	inside_gates_nodes as(
	   select in_node,
			  out_node,
			  1 as level,
			  Array[in_node] as circle_check
	   from all_nodes
	   where in_node=$2
	   union all
	   select a.in_node,
			  a.out_node,
			  case when a.out_node like 'wait_for_all_inputs%' then ign.level+1
				   when a.out_node like 'begin_parallel_task%' then ign.level-1
				   else ign.level end as level,
			  array_append(ign.circle_check, a.in_node)
	   from all_nodes a
				inner join inside_gates_nodes ign on a.in_node=ign.out_node
	   where array_position(circle_check,a.out_node) is null and
			   a.in_node not like 'begin_parallel_task%' and ign.level!=0)
	select
    (
        select case when count(*)=0 then true else false end
        from variable_storage vs
                 inner join works w on vs.work_id = w.id
                 inner join inside_gates_nodes ign on vs.step_name=ign.out_node
        where w.work_number=$1 and w.child_id is null and vs.status in('running', 'idle', 'ready')
    ) as is_finished,
    (
        select case when count(distinct vs.step_name) = 
			(select count(distinct inside_gates_nodes.in_node) 
				from inside_gates_nodes 
			where out_node like 'begin_parallel_task_%')
        then true else false end
    	from variable_storage vs
                 inner join works w on vs.work_id = w.id
                 inner join inside_gates_nodes ign on vs.step_name=ign.in_node
        where w.work_number=$1 and w.child_id is null and ign.out_node like 'begin_parallel_task_%'
	) as created_all_branches`

	var parallelIsFinished bool
	var createdAllBranches bool
	row := db.Connection.QueryRow(ctx, q, workNumber, blockName)

	if err := row.Scan(&parallelIsFinished, &createdAllBranches); err != nil {
		return false, err
	}

	return parallelIsFinished && createdAllBranches, nil
}

func (db *PGCon) GetTaskStepById(ctx context.Context, id uuid.UUID) (*entity.Step, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_task_step_by_id")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := `
	SELECT 
	    vs.id,
	    vs.work_id,
		w.work_number,
	    vs.step_type,
		vs.step_name, 
		vs.time, 
		vs.content, 
		COALESCE(vs.break_points, '{}') AS break_points, 
		vs.has_error,
		vs.status,
		w.author,
		vs.updated_at,
		w.run_context -> 'initial_application' -> 'is_test_application' as isTest
	FROM variable_storage vs 
	JOIN works w ON vs.work_id = w.id
		WHERE vs.id = $1
	LIMIT 1`

	var s entity.Step
	var content string
	err := db.Connection.QueryRow(ctx, q, id).Scan(
		&s.ID,
		&s.WorkID,
		&s.WorkNumber,
		&s.Type,
		&s.Name,
		&s.Time,
		&content,
		&s.BreakPoints,
		&s.HasError,
		&s.Status,
		&s.Initiator,
		&s.UpdatedAt,
		&s.IsTest,
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
		ORDER BY vs.time DESC
		LIMIT 1
`

	var s entity.Step
	var content string
	queryErr := db.Connection.QueryRow(ctx, query, workID, stepName).Scan(
		&s.ID,
		&s.Type,
		&s.Name,
		&s.Time,
		&content,
		&s.BreakPoints,
		&s.HasError,
		&s.Status,
	)
	if queryErr != nil {
		return nil, queryErr
	}

	storage := store.NewStore()

	if unmarshalErr := json.Unmarshal([]byte(content), storage); unmarshalErr != nil {
		return nil, unmarshalErr
	}

	s.State = storage.State
	s.Steps = storage.Steps
	s.Errors = storage.Errors
	s.Storage = storage.Values

	return &s, nil
}

func (db *PGCon) GetCanceledTaskSteps(ctx context.Context, taskID uuid.UUID) ([]entity.Step, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_cancelled_task_steps")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	const query = `
		SELECT
			vs.step_name, 
			vs.time
		FROM variable_storage vs  
			WHERE vs.work_id = $1 AND vs.status = 'cancel'`

	rows, err := db.Connection.Query(ctx, query, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make([]entity.Step, 0)
	for rows.Next() {
		s := entity.Step{}
		if scanErr := rows.Scan(&s.Name, &s.Time); scanErr != nil {
			return nil, scanErr
		}
		res = append(res, s)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return res, nil
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
			ORDER BY vs.time DESC
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
		AND pv.deleted_at IS NULL
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

func (db *PGCon) GetVersionByPipelineID(c context.Context, pipelineID string) (*entity.EriusScenario, error) {
	c, span := trace.StartSpan(c, "pg_get_version_by_pipeline_id")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	const query = `
	SELECT pv.id,
       pv.status,
       pv.pipeline_id,
       pv.created_at,
       pv.content,
       pv.comment_rejected,
       pv.comment,
       pv.author,
       vs.start_schema,
       vs.end_schema,
       (SELECT MAX(date) FROM pipeline_history WHERE pipeline_id = pv.pipeline_id) AS last_approve
	FROM versions pv
			 LEFT JOIN pipelines p ON pv.pipeline_id = p.id
			 LEFT JOIN version_settings vs on pv.id = vs.version_id
	WHERE pv.status = 2
	  AND pv.is_actual = TRUE
	  AND pv.pipeline_id = $1
	  AND p.deleted_at IS NULL
`
	res := &entity.EriusScenario{}

	var (
		vID, pID uuid.UUID
		s        int
		content  string
		cr       string
		cm       string
		d        *time.Time
		ca       *time.Time
		ss, es   *script.JSONSchema
		a        string
	)

	err := db.Connection.QueryRow(c, query, pipelineID).Scan(&vID, &s, &pID, &ca, &content, &cr, &cm, &a, &ss, &es, &d)
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal([]byte(content), res); err != nil {
		return nil, err
	}

	res.VersionID = vID
	res.ID = pID
	res.Status = s
	res.CommentRejected = cr
	res.Comment = cm
	res.ApprovedAt = d
	res.CreatedAt = ca
	res.Settings.StartSchema = ss
	res.Settings.EndSchema = es
	res.Author = a

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
		pipelineName := strings.Replace(*dto.PipelineName, "_", "!_", -1)
		pipelineName = strings.Replace(pipelineName, "%", "!%", -1)
		q = strings.ReplaceAll(q, "--pipe--", fmt.Sprintf("AND p.name ilike'%%%s%%' ESCAPE '!'", pipelineName))
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
    where ((step_type = 'approver' and content -> 'State' -> step_name -> 'approvers' ? $3)
        or (step_type = 'execution' and content -> 'State' -> step_name -> 'executors' ? $3)
        or (step_type = 'form' and content -> 'State' -> step_name -> 'executors' ? $3)
		or (step_type = 'sign' and content -> 'State' -> step_name -> 'signers' ? $3))
      and work_id = (SELECT id
                     FROM works
                     WHERE work_number = $1
                       AND child_id IS NULL
    )
)
select count(*)
from accesses
where accesses.data::jsonb ->> 'node_id' = $2
  and accesses.data::jsonb ->> 'accessType' = 'ReadWrite'
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

	// nolint:gocritic
	// language=PostgreSQL
	const q = `
		SELECT run_context
		FROM works
		WHERE work_number = $1 AND child_id IS NULL`

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
		WHERE w.work_number = $2 AND w.child_id IS NULL`

	var f *entity.EriusFunc

	if scanErr := db.Connection.QueryRow(ctx, q, blockName, workNumber).Scan(&f); scanErr != nil {
		return nil, scanErr
	}

	if f == nil {
		return nil, errors.New("couldn't find block data")
	}
	return f, nil
}

func (db *PGCon) StopTaskBlocks(ctx context.Context, taskID uuid.UUID) error {
	ctx, span := trace.StartSpan(ctx, "stop_task_blocks")
	defer span.End()

	q := `
		UPDATE variable_storage
		SET status = 'cancel', updated_at = now()
		WHERE work_id = $1 AND status IN ('ready', 'idle', 'running')`

	_, err := db.Connection.Exec(ctx, q, taskID)
	return err
}

func (db *PGCon) GetVariableStorageForStep(ctx context.Context, taskID uuid.UUID, stepType string) (*store.VariableStore, error) {
	ctx, span := trace.StartSpan(ctx, "get_variable_storage_for_step")
	defer span.End()

	const q = `
		SELECT content
		FROM variable_storage
		WHERE work_id = $1 AND step_name = $2 
		ORDER BY time DESC LIMIT 1`

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

	// nolint:gocritic
	// language=PostgreSQL
	// half_sla = (vs.time + (vs.sla_deadline - vs.time)/2)
	q := `
		SELECT w.id,
		       w.work_number,
		       p.name,	
		       w.author,
		       vs.content,
		       v.content->'pipeline'->'blocks'->vs.step_name,
		       vs.step_name,
		       d.action,
			   w.run_context -> 'initial_application' -> 'is_test_application' as isTest
		FROM variable_storage vs 
		    JOIN works w on vs.work_id = w.id 
		    JOIN versions v on w.version_id = v.id
			JOIN pipelines p on v.pipeline_id = p.id
		    JOIN deadlines d on vs.id = d.block_id
		WHERE (
		    vs.status = 'running' 
		    OR ( vs.status = 'idle' AND (
		    d.action = 'rework_sla_breached' OR d.action = 'day_before_sla_request_add_info' OR d.action = 'sla_breach_request_add_info'))
			)
			AND w.child_id IS NULL
			AND d.deadline < NOW()`
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
			&item.Action,
			&item.IsTest,
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

func (db *PGCon) GetTaskForMonitoring(ctx context.Context, workNumber string) ([]entity.MonitoringTaskNode, error) {
	ctx, span := trace.StartSpan(ctx, "get_task_nodes_for_monitoring")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := `
		SELECT w.work_number, 
		       w.version_id, 
		       p.author,
		       p.created_at::text,
		       p.name,
		       vs.step_name, 
		       vs.status,
		       vs.id,
       		   v.content->'pipeline'-> 'blocks'->step_name->>'title' title,
       		   vs.time block_date_init
		from works w
    		join versions v on w.version_id = v.id
    		join pipelines p on v.pipeline_id = p.id
    		join variable_storage vs on w.id = vs.work_id
		where w.work_number = $1
		order by vs.time`

	rows, err := db.Connection.Query(ctx, q, workNumber)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := make([]entity.MonitoringTaskNode, 0)
	for rows.Next() {
		item := entity.MonitoringTaskNode{}
		if scanErr := rows.Scan(
			&item.WorkNumber,
			&item.VersionId,
			&item.Author,
			&item.CreationTime,
			&item.ScenarioName,
			&item.NodeId,
			&item.Status,
			&item.BlockId,
			&item.RealName,
			&item.BlockDateInit,
		); scanErr != nil {
			return nil, scanErr
		}

		res = append(res, item)
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}
	return res, nil
}

func (db *PGCon) GetVersionSettings(ctx context.Context, versionID string) (entity.ProcessSettings, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_version_settings")
	defer span.End()

	// nolint:gocritic,lll
	// language=PostgreSQL
	query := `
	SELECT start_schema, end_schema, resubmission_period,
	       (select p.name from pipelines p where p.id = 
	                                             (select pipeline_id from versions v where v.id = 
	                                                                              (select version_id from version_settings vs where vs.id = version_settings.id
	                                                                                                                          )
	                                                                        )
	                                       ) "name"
	FROM version_settings
	WHERE version_id = $1`

	row := db.Connection.QueryRow(ctx, query, versionID)

	processSettings := entity.ProcessSettings{Id: versionID}
	err := row.Scan(&processSettings.StartSchema, &processSettings.EndSchema, &processSettings.ResubmissionPeriod, &processSettings.Name)
	if err != nil && err != pgx.ErrNoRows {
		return processSettings, err
	}

	return processSettings, nil
}

func (db *PGCon) SaveVersionSettings(ctx context.Context, settings entity.ProcessSettings, schemaFlag *string) error {
	ctx, span := trace.StartSpan(ctx, "pg_save_version_settings")
	defer span.End()

	var (
		commandTag pgconn.CommandTag
		err        error
	)

	if schemaFlag == nil {
		// nolint:gocritic
		// language=PostgreSQL
		query := `
		INSERT INTO version_settings (id, version_id, start_schema, end_schema) 
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (version_id) DO UPDATE 
			SET start_schema = excluded.start_schema, 
				end_schema = excluded.end_schema`
		commandTag, err = db.Connection.Exec(ctx,
			query,
			uuid.New(),
			settings.Id,
			settings.StartSchema,
			settings.EndSchema,
		)
		if err != nil {
			return err
		}
	} else {
		var jsonSchema *script.JSONSchema
		switch *schemaFlag {
		case startSchema:
			jsonSchema = settings.StartSchema
		case endSchema:
			jsonSchema = settings.EndSchema
		default:
			return errUnkonwnSchemaFlag
		}

		// nolint:gocritic
		// language=PostgreSQL
		query := fmt.Sprintf(`INSERT INTO version_settings (id, version_id, %[1]s) 
			VALUES ($1, $2, $3)
			ON CONFLICT (version_id) DO UPDATE 
				SET %[1]s = excluded.%[1]s`, *schemaFlag)

		commandTag, err = db.Connection.Exec(ctx, query, uuid.New(), settings.Id, jsonSchema)
		if err != nil {
			return err
		}
	}

	if commandTag.RowsAffected() != 0 {
		_ = db.RemoveObsoleteMapping(ctx, settings.Id)
	}

	return nil
}

func (db *PGCon) SaveVersionMainSettings(ctx context.Context, params entity.ProcessSettings) error {
	ctx, span := trace.StartSpan(ctx, "pg_save_version_main_settings")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	query := `INSERT INTO version_settings (id, version_id, resubmission_period) 
			VALUES ($1, $2, $3)
			ON CONFLICT (version_id) DO UPDATE 
			SET resubmission_period = excluded.resubmission_period`

	_, err := db.Connection.Exec(ctx, query, uuid.New(), params.Id, params.ResubmissionPeriod)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGCon) AddExternalSystemToVersion(ctx context.Context, versionID, systemID string) error {
	ctx, span := trace.StartSpan(ctx, "pg_add_external_system_to_version")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	query := `INSERT INTO external_systems (id, version_id, system_id) VALUES ($1, $2, $3)`

	_, err := db.Connection.Exec(ctx, query, uuid.New(), versionID, systemID)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGCon) GetExternalSystemsIDs(ctx context.Context, versionID string) ([]uuid.UUID, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_external_systems_ids")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	query := `
	SELECT array_agg(system_id)
	FROM external_systems
	WHERE version_id = $1`

	row := db.Connection.QueryRow(ctx, query, versionID)

	var systemIDs []uuid.UUID
	err := row.Scan(&systemIDs)
	if err != nil {
		return nil, err
	}

	return systemIDs, nil
}

func (db *PGCon) GetTaskEventsParamsByWorkNumber(ctx context.Context, workNumber,
	systemID string) (entity.ExternalSystemSubscriptionParams, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_task_events_params_by_work_number")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	query := `
	SELECT system_id, microservice_id, path,
	method, notification_schema, mapping, nodes
	FROM external_system_task_subscriptions
	WHERE version_id = (SELECT version_id FROM works WHERE work_number = $1 limit 1) AND system_id = $2`

	row := db.Connection.QueryRow(ctx, query, workNumber, systemID)

	params := entity.ExternalSystemSubscriptionParams{
		NotificationSchema: script.JSONSchema{},
		Mapping:            script.JSONSchemaProperties{},
		Nodes:              make([]entity.NodeSubscriptionEvents, 0),
	}
	err := row.Scan(
		&params.SystemID,
		&params.MicroserviceID,
		&params.Path,
		&params.Method,
		&params.NotificationSchema,
		&params.Mapping,
		&params.Nodes,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.ExternalSystemSubscriptionParams{}, nil
		}
		return params, err
	}

	return params, nil
}

func (db *PGCon) GetExternalSystemTaskSubscriptions(ctx context.Context, versionID,
	systemID string) (entity.ExternalSystemSubscriptionParams, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_external_system_task_subscriptions")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	query := `
	SELECT system_id, microservice_id, path,
	method, notification_schema, mapping, nodes
	FROM external_system_task_subscriptions
	WHERE version_id = $1 AND system_id = $2`

	row := db.Connection.QueryRow(ctx, query, versionID, systemID)

	params := entity.ExternalSystemSubscriptionParams{
		NotificationSchema: script.JSONSchema{},
		Mapping:            script.JSONSchemaProperties{},
		Nodes:              make([]entity.NodeSubscriptionEvents, 0),
	}
	err := row.Scan(
		&params.SystemID,
		&params.MicroserviceID,
		&params.Path,
		&params.Method,
		&params.NotificationSchema,
		&params.Mapping,
		&params.Nodes,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.ExternalSystemSubscriptionParams{}, nil
		}
		return params, err
	}

	return params, nil
}

func (db *PGCon) GetExternalSystemSettings(ctx context.Context, versionID, systemID string) (entity.ExternalSystem, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_external_system_settings")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	query := `
	SELECT input_schema, output_schema, input_mapping, output_mapping,
	microservice_id, ending_url, sending_method, allow_run_as_others
	FROM external_systems
	WHERE version_id = $1 AND system_id = $2`

	row := db.Connection.QueryRow(ctx, query, versionID, systemID)

	externalSystemSettings := entity.ExternalSystem{Id: systemID, OutputSettings: &entity.EndSystemSettings{}}
	err := row.Scan(
		&externalSystemSettings.InputSchema,
		&externalSystemSettings.OutputSchema,
		&externalSystemSettings.InputMapping,
		&externalSystemSettings.OutputMapping,
		&externalSystemSettings.OutputSettings.MicroserviceId,
		&externalSystemSettings.OutputSettings.URL,
		&externalSystemSettings.OutputSettings.Method,
		&externalSystemSettings.AllowRunAsOthers,
	)
	if err != nil {
		return externalSystemSettings, err
	}

	return externalSystemSettings, nil
}

func (db *PGCon) SaveExternalSystemSubscriptionParams(ctx context.Context, versionID string,
	params *entity.ExternalSystemSubscriptionParams) error {
	ctx, span := trace.StartSpan(ctx, "pg_save_external_system_subscription_params")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := `INSERT INTO external_system_task_subscriptions 
    (id, version_id, system_id, microservice_id, path, method, notification_schema, mapping, nodes) 
    values 
    ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err := db.Connection.Exec(ctx, q, uuid.New().String(), versionID, params.SystemID, params.MicroserviceID,
		params.Path, params.Method, params.NotificationSchema, params.Mapping, params.Nodes)
	if err != nil {
		return err
	}
	return nil
}

func (db *PGCon) SaveExternalSystemSettings(
	ctx context.Context, versionID string, system entity.ExternalSystem, schemaFlag *string) error {
	ctx, span := trace.StartSpan(ctx, "pg_save_external_system_settings")
	defer span.End()

	args := []interface{}{versionID, system.Id}
	var schemasForUpdate string
	if schemaFlag != nil {
		switch *schemaFlag {
		case inputSchema:
			schemasForUpdate = inputSchema + " = $3"
			args = append(args, system.InputSchema)
		case outputSchema:
			schemasForUpdate = outputSchema + " = $3"
			args = append(args, system.OutputSchema)
		case inputMapping:
			schemasForUpdate = inputMapping + " = $3"
			args = append(args, system.InputMapping)
		case outputMapping:
			schemasForUpdate = outputMapping + " = $3"
			args = append(args, system.OutputMapping)
		default:
			return errUnkonwnSchemaFlag
		}
	} else {
		schemasForUpdate = "input_schema = $3, output_schema = $4, input_mapping = $5, output_mapping = $6"
		args = append(args, system.InputSchema, system.OutputSchema, system.InputMapping, system.OutputMapping)
	}

	// nolint:gocritic
	// language=PostgreSQL
	query := fmt.Sprintf(`UPDATE external_systems
		SET %s
		WHERE version_id = $1 AND system_id = $2`, schemasForUpdate)

	commandTag, err := db.Connection.Exec(ctx, query, args...)
	if err != nil {
		return err
	}

	if commandTag.RowsAffected() == 0 {
		return errCantFindExternalSystem
	}

	return nil
}

func (db *PGCon) RemoveExternalSystemTaskSubscriptions(ctx context.Context, versionID, systemID string) error {
	ctx, span := trace.StartSpan(ctx, "pg_remove_external_system_task_subscriptions")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	query := `DELETE FROM external_system_task_subscriptions WHERE version_id = $1`
	args := []interface{}{versionID}
	if systemID != "" {
		query += " AND system_id = $2"
		args = append(args, systemID)
	}

	_, err := db.Connection.Exec(ctx, query, args...)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGCon) RemoveExternalSystem(ctx context.Context, versionID, systemID string) error {
	ctx, span := trace.StartSpan(ctx, "pg_remove_external_system")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	query := `DELETE FROM external_systems WHERE version_id = $1 AND system_id = $2`

	_, err := db.Connection.Exec(ctx, query, versionID, systemID)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGCon) RemoveObsoleteMapping(ctx context.Context, versionID string) error {
	ctx, span := trace.StartSpan(ctx, "pg_remove_obsolete_mapping")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	query := `UPDATE external_systems
		SET input_mapping = NULL, output_mapping = NULL
		WHERE version_id = $1`

	_, err := db.Connection.Exec(ctx, query, versionID)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGCon) GetWorksForUserWithGivenTimeRange(
	ctx context.Context,
	hours int,
	login,
	versionID,
	excludeWorkNumber string) ([]*entity.EriusTask, error) {
	ctx, span := trace.StartSpan(ctx, "get_works_for_user_with_given_time_range")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	query := `WITH works_cte as (select w.id work_id,
                          w.author work_author,
                          (w.run_context -> 'initial_application' -> 'application_body' -> 'recipient' ->>
                           'username') work_recipient,
                          w.work_number work_number,
                          w.started_at work_started
                   from works w
                     where w.started_at > now() - interval '1 hour' * $1
                     and w.version_id = $2
                     and w.work_number != $3
                     and w.child_id is null)
				   select work_id, work_author, work_number, work_started
				   from works_cte
				   where works_cte.work_recipient = $4
				   or (works_cte.work_recipient is null and works_cte.work_author = $4)`

	rows, queryErr := db.Connection.Query(ctx, query, hours, versionID, excludeWorkNumber, login)
	if queryErr != nil {
		return nil, queryErr
	}
	defer rows.Close()

	works := make([]*entity.EriusTask, 0)

	for rows.Next() {
		var work entity.EriusTask

		scanErr := rows.Scan(
			&work.ID,
			&work.Author,
			&work.WorkNumber,
			&work.StartedAt,
		)
		if scanErr != nil {
			return nil, scanErr
		}
		works = append(works, &work)
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}

	return works, nil
}

func (db *PGCon) CheckPipelineNameExists(ctx context.Context, name string, checkNotDeleted bool) (*bool, error) {
	c, span := trace.StartSpan(ctx, "check_pipeline_name_exists")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	qVersion := `
	select exists(
	    select 1 from pipelines where name = $1 --is_deleted--
	    )`

	if checkNotDeleted {
		qVersion = strings.Replace(qVersion, "--is_deleted--", "AND pipelines.deleted_at IS NULL", 1)
	}

	row := db.Connection.QueryRow(c, qVersion, name)

	var pipelineNameExists bool

	scanErr := row.Scan(&pipelineNameExists)

	if scanErr != nil {
		return nil, scanErr
	}

	return &pipelineNameExists, nil
}

func (db *PGCon) UpdateEndingSystemSettings(ctx context.Context, versionID, systemID string, s entity.EndSystemSettings) (err error) {
	ctx, span := trace.StartSpan(ctx, "pg_update_ending_system_settings")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	query := `
	UPDATE external_systems
	SET (microservice_id, ending_url, sending_method) = ($1, $2, $3)
	WHERE version_id = $4 AND system_id = $5`

	_, err = db.Connection.Exec(ctx, query, s.MicroserviceId, s.URL, s.Method, versionID, systemID)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGCon) AllowRunAsOthers(ctx context.Context, versionID, systemID string, allowRunAsOthers bool) (err error) {
	ctx, span := trace.StartSpan(ctx, "pg_allow_run_as_others")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	query := `
	UPDATE external_systems
	SET allow_run_as_others = $1
	WHERE version_id = $2 AND system_id = $3`

	commandTag, err := db.Connection.Exec(ctx, query, allowRunAsOthers, versionID, systemID)
	if err != nil {
		return err
	}

	if commandTag.RowsAffected() == 0 {
		return errCantFindExternalSystem
	}

	return nil
}

func (db *PGCon) GetTaskInWorkTime(ctx context.Context, workNumber string) (*entity.TaskCompletionInterval, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_task_in_work_time")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	query := `
	SELECT started_at, finished_at
	FROM works
	WHERE work_number = $1`

	row := db.Connection.QueryRow(ctx, query, workNumber)

	interval := entity.TaskCompletionInterval{}
	err := row.Scan(
		&interval.StartedAt,
		&interval.FinishedAt,
	)
	if err != nil {
		return &entity.TaskCompletionInterval{}, err
	}

	return &interval, nil
}

func (db *PGCon) SaveSlaVersionSettings(ctx context.Context, versionID string, s entity.SlaVersionSettings) (err error) {
	ctx, span := trace.StartSpan(ctx, "pg_save_sla_version_settings")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	query := `
	INSERT INTO version_sla (id, version_id, author, created_at, work_type, sla)
	VALUES ( $1, $2, $3, now(), $4, $5)`

	_, err = db.Connection.Exec(ctx, query, uuid.New(), versionID, s.Author, s.WorkType, s.Sla)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGCon) GetSlaVersionSettings(ctx context.Context, versionID string) (s entity.SlaVersionSettings, err error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_sla_version_settings")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	query := `
	SELECT author, work_type, sla
	FROM version_sla
	WHERE version_id = $1
	ORDER BY created_at DESC`

	row := db.Connection.QueryRow(ctx, query, versionID)
	slaSettings := entity.SlaVersionSettings{}
	err = row.Scan(
		&slaSettings.Author,
		&slaSettings.WorkType,
		&slaSettings.Sla,
	)
	if err != nil {
		return entity.SlaVersionSettings{}, err
	}
	return slaSettings, nil
}
