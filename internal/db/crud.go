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

	"github.com/google/uuid"

	"github.com/iancoleman/orderedmap"

	"github.com/jackc/pgconn"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"

	"github.com/lib/pq"

	"go.opencensus.io/trace"

	"golang.org/x/exp/slices"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/configs"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type PGCon struct {
	Connection Connector
}

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

func (db *PGCon) Acquire(ctx context.Context) (Database, error) {
	_, span := trace.StartSpan(ctx, "acquire_conn")
	defer span.End()

	if acConn, ok := db.Connection.(interface {
		Acquire(ctx context.Context) (*pgxpool.Conn, error)
	}); ok {
		ac, err := acConn.Acquire(ctx)
		if err != nil {
			return nil, err
		}

		return &PGCon{Connection: ac}, nil
	}

	return nil, errors.New("can't acquire connection")
}

func (db *PGCon) Release(ctx context.Context) error {
	_, span := trace.StartSpan(ctx, "release_conn")
	defer span.End()

	if releaseConn, ok := db.Connection.(interface {
		Release()
	}); ok {
		releaseConn.Release()

		return nil
	}

	return errors.New("can't release connection")
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

func (db *PGCon) GetPipelinesWithLatestVersion(
	c context.Context,
	authorLogin string,
	publishedPipelines bool,
	page, perPage *int,
	filter string,
) ([]entity.EriusScenarioInfo, error) {
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
		//nolint:gocritic,goconst //я не одобряю такие шайтан фокусы с sql но трогать работающий код не очень хочется
		q = strings.ReplaceAll(q, "---author---", "AND pv.author='"+authorLogin+"'")
	}

	if filter != "" {
		escapeFilter := strings.ReplaceAll(filter, "_", "!_")
		escapeFilter = strings.ReplaceAll(escapeFilter, "%", "!%")
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

	defer rows.Close()

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
		p.PipelineID = pID
		p.Status = s
		p.Name = name
		pipes = append(pipes, p)
	}

	return pipes, nil
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
	p *entity.EriusScenario, author string, pipelineData []byte, oldVersionID uuid.UUID, hasPrivateFunction bool,
) error {
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

	_, err := db.Connection.Exec(c, qNewPipeline, p.PipelineID, p.Name, createdAt, author)
	if err != nil {
		return err
	}

	return db.CreateVersion(c, p, author, pipelineData, oldVersionID, hasPrivateFunction)
}

func (db *PGCon) CreateVersion(c context.Context,
	p *entity.EriusScenario, author string, pipelineData []byte, oldVersionID uuid.UUID, isHidden bool,
) error {
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
		updated_at,
	    is_hidden
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
	)`

	createdAt := time.Now()

	_, err := db.Connection.Exec(c,
		qNewVersion,
		p.VersionID,
		StatusDraft,
		p.PipelineID,
		createdAt,
		pipelineData,
		author,
		p.Comment,
		createdAt,
		isHidden)
	if err != nil {
		return err
	}

	if oldVersionID != uuid.Nil {
		err = db.copyProcessSettingsFromOldVersion(c, p.VersionID, oldVersionID)
		if err != nil {
			return err
		}
	} else {
		err = db.SaveVersionSettings(c, entity.ProcessSettings{VersionID: p.VersionID.String(), ResubmissionPeriod: 0}, nil)
		if err != nil {
			return err
		}
		err = db.SaveSLAVersionSettings(c, p.VersionID.String(), entity.SLAVersionSettings{
			Author:   author,
			WorkType: "8/5",
			SLA:      40,
		})
		if err != nil {
			return err
		}
	}

	return nil
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

	//nolint:errcheck // tx.Rollback() error ignored, невкусно обрабатывать эту ошибку
	defer tx.Rollback(c)

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
		p.PipelineID = pID
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
		p.PipelineID = pID
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
          (SELECT id 
           FROM versions ver 
           WHERE ver.pipeline_id = $2 ORDER BY created_at DESC LIMIT 1) 
    ;`

	_, err := db.Connection.Exec(c, query, name, id)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGCon) UpdateDraft(
	c context.Context,
	p *entity.EriusScenario,
	pipelineData []byte,
	groups []*entity.NodeGroup,
	isHidden bool,
) error {
	c, span := trace.StartSpan(c, "pg_update_draft")
	defer span.End()

	tx, err := db.Connection.Begin(c)
	if err != nil {
		return err
	}

	//nolint:errcheck // tx.Rollback() error ignored, невкусно обрабатывать эту ошибку
	defer tx.Rollback(c)

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
		node_groups = $6,
		is_hidden = $7
	WHERE id = $8`

	_, err = tx.Exec(c, q, p.Status, pipelineData, p.Comment, p.Status == StatusApproved, time.Now(), groups, isHidden, p.VersionID)
	if err != nil {
		return err
	}

	if p.Status == StatusApproved {
		q = `
	UPDATE versions
	SET is_actual = FALSE
	WHERE id != $1
	AND pipeline_id = $2`

		_, err = tx.Exec(c, q, p.VersionID, p.PipelineID)
		if err != nil {
			return err
		}
	}

	return tx.Commit(c)
}

func (db *PGCon) UpdateGroupsForEmptyVersions(
	c context.Context,
	versionID string,
	groups []*entity.NodeGroup,
) error {
	c, span := trace.StartSpan(c, "pg_update_groups_for_empty_versions")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := `
	UPDATE versions 
	SET 
		node_groups = $1
	WHERE id = $2`

	_, err := db.Connection.Exec(c, q, groups, versionID)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGCon) IsStepExist(ctx context.Context, workID, stepName string, hasUpdData bool) (bool, uuid.UUID, time.Time, error) {
	var (
		id uuid.UUID
		t  time.Time
	)

	formStatuses := "('idle', 'running')"
	if hasUpdData {
		formStatuses = "('idle', 'running','finished')"
	}

	q := `
		SELECT id, time
		FROM variable_storage
		WHERE work_id = $1 AND
			step_name = $2 AND
			(((status IN ('idle', 'running') AND is_paused = false) OR (status = 'ready')) OR (
				step_type = 'form' AND
				((status IN --formStatuses-- AND is_paused = false) OR (status = 'ready')) AND
				time = (SELECT max(time) FROM variable_storage vs 
							WHERE vs.work_id = $1 AND step_name = $2)
			))
		FOR UPDATE`

	q = strings.Replace(q, "--formStatuses--", formStatuses, 1)

	scanErr := db.Connection.QueryRow(ctx, q, workID, stepName).Scan(&id, &t)
	if scanErr != nil && !errors.Is(scanErr, pgx.ErrNoRows) {
		return false, uuid.Nil, time.Time{}, scanErr
	}

	return id != uuid.Nil, id, t, nil
}

func (db *PGCon) InitTaskBlock(
	ctx context.Context,
	dto *SaveStepRequest,
	isPaused, hasUpdData bool,
) (id uuid.UUID, startTime time.Time, err error) {
	ctx, span := trace.StartSpan(ctx, "pg_init_task_block")
	defer span.End()

	exists, stepID, t, existErr := db.IsStepExist(ctx, dto.WorkID.String(), dto.StepName, hasUpdData)
	if existErr != nil {
		return uuid.Nil, time.Time{}, existErr
	}

	if exists {
		return stepID, t, nil
	}

	id = uuid.New()

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
			status,
		    attachments, 	                         
		    current_executor,
			is_active,
		    is_paused
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
		    true,
		    $12
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
		dto.Attachments,
		dto.CurrentExecutor,
		isPaused,
	}

	_, err = db.Connection.Exec(ctx, query, args...)
	if err != nil {
		return uuid.Nil, time.Time{}, err
	}

	return id, timestamp, nil
}

func (db *PGCon) CopyTaskBlock(ctx context.Context, stepID uuid.UUID) (newStepID uuid.UUID, err error) {
	ctx, span := trace.StartSpan(ctx, "pg_copy_task_block")
	defer span.End()

	newStepID = uuid.New()

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
			attachments, 	                         
			current_executor,
			is_active,
			is_paused
		)
		SELECT
			$1,
			work_id,
			step_type,
			step_name, 
			content, 
			now(), 
			break_points, 
			false,
			status,
			attachments, 	                         
			current_executor,
			true,
			false
		FROM variable_storage 
		WHERE id = $2`

	_, err = db.Connection.Exec(ctx, query, newStepID, stepID)
	if err != nil {
		return newStepID, err
	}

	return newStepID, nil
}

func (db *PGCon) SaveStepContext(ctx context.Context, dto *SaveStepRequest, id uuid.UUID) (uuid.UUID, error) {
	ctx, span := trace.StartSpan(ctx, "pg_save_step_context")
	defer span.End()

	if !dto.IsReEntry && dto.BlockExist {
		exists, stepID, _, err := db.IsStepExist(ctx, dto.WorkID.String(), dto.StepName, false)
		if err != nil {
			return uuid.Nil, err
		}

		if exists {
			return stepID, nil
		}
	}
	// nolint:gocritic
	// language=PostgreSQL
	query := `
		UPDATE variable_storage SET 
			content = $2, 
			break_points = $3, 
			has_error = $4,
			status = $5,
		    attachments = $6, 	                         
		    current_executor = $7,
			is_paused = $8
			--update_col--
			WHERE id = $1
`
	args := []interface{}{
		id,
		dto.Content,
		dto.BreakPoints,
		dto.HasError,
		dto.Status,
		dto.Attachments,
		dto.CurrentExecutor,
		false,
	}

	if _, ok := map[string]struct{}{"finished": {}, "no_success": {}, "error": {}}[dto.Status]; ok {
		args = append(args, time.Now())
		query = strings.Replace(query, "--update_col--", fmt.Sprintf(",updated_at = $%d", len(args)), 1)
	}

	_, err := db.Connection.Exec(ctx, query, args...)
	if err != nil {
		return uuid.Nil, err
	}

	err = db.insertIntoMembers(ctx, dto.Members, id)
	if err != nil {
		return uuid.Nil, err
	}

	err = db.deleteAndInsertIntoDeadlines(ctx, dto.Deadlines, id)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func (db *PGCon) UpdateStepContext(ctx context.Context, dto *UpdateStepRequest) error {
	c, span := trace.StartSpan(ctx, "pg_update_step_context")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	const q = `
		UPDATE variable_storage
		SET
			break_points = $2
			, has_error = $3
			, status = $4
			, content = $5
			, attachments = $6
		    , current_executor = $7
			, updated_at = NOW()
		WHERE id = $1`

	args := []interface{}{
		dto.ID, dto.BreakPoints, dto.HasError, dto.Status, dto.Content, dto.Attachments, dto.CurrentExecutor,
	}

	_, err := db.Connection.Exec(c, q, args...)
	if err != nil {
		return err
	}

	_, delSpan := trace.StartSpan(ctx, "pg_delete_block_members")
	defer delSpan.End()

	// nolint:gocritic
	// language=PostgreSQL
	const qMembersDelete = `
		DELETE FROM members 
		WHERE block_id = $1`

	_, err = db.Connection.Exec(ctx, qMembersDelete, dto.ID)
	if err != nil {
		return err
	}

	err = db.insertIntoMembers(ctx, dto.Members, dto.ID)
	if err != nil {
		return err
	}

	err = db.deleteAndInsertIntoDeadlines(ctx, dto.Deadlines, dto.ID)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGCon) insertIntoMembers(ctx context.Context, members []Member, id uuid.UUID) error {
	_, span := trace.StartSpan(ctx, "pg_insert_into_members")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	const queryMembers = `
		INSERT INTO members (
			id,
			block_id,
			login,
			actions,
		    params,
		    is_acted,
		    execution_group_member,
			is_initiator,
		    finished
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
		)`

	for _, val := range members {
		membersID := uuid.New()
		actions := make(pq.StringArray, 0, len(val.Actions))
		params := make(map[string]map[string]interface{})

		for _, act := range val.Actions {
			actions = append(actions, act.ID+":"+act.Type)

			if len(act.Params) != 0 {
				params[act.ID] = act.Params
			}
		}

		paramsData, mErr := json.Marshal(params)
		if mErr != nil {
			return mErr
		}

		_, err := db.Connection.Exec(
			ctx,
			queryMembers,
			membersID,
			id,
			val.Login,
			actions,
			paramsData,
			val.IsActed,
			val.ExecutionGroupMember,
			val.IsInitiator,
			val.Finished,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (db *PGCon) insertIntoDeadlines(ctx context.Context, deadlines []Deadline, id uuid.UUID) error {
	_, span := trace.StartSpan(ctx, "pg_create_block_deadlines")
	defer span.End()

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
		deadlineID := uuid.New()

		_, err := db.Connection.Exec(
			ctx,
			queryDeadlines,
			deadlineID,
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
	_, span := trace.StartSpan(ctx, "pg_delete_block_deadlines")
	defer span.End()

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

func (db *PGCon) deleteAndInsertIntoDeadlines(ctx context.Context, deadlines []Deadline, id uuid.UUID) error {
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

	defer rows.Close()

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
		p.PipelineID = pID
		p.Status = s
		p.Name = name
		p.ApprovedAt = &d
		pipes = append(pipes, p)
	}

	vMap := make(map[uuid.UUID]entity.EriusScenario)

	for i := range pipes {
		version := pipes[i]
		if finV, ok := vMap[version.PipelineID]; ok {
			t, err := db.findApproveDate(c, version.VersionID)
			if err != nil {
				return nil, err
			}

			if finV.ApprovedAt.After(t) {
				continue
			}
		}

		vMap[version.PipelineID] = version
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
		p.PipelineID = pID
		p.Status = s

		return &p, nil
	}

	return nil, nil
}

func (db *PGCon) GetUnfinishedTaskSteps(ctx context.Context, in *entity.GetUnfinishedTaskSteps) (entity.TaskSteps, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_unfinished_task_steps")
	defer span.End()

	el := entity.TaskSteps{}

	var notInStatuses []string

	isAddInfoReq := slices.Contains([]entity.TaskUpdateAction{
		entity.TaskUpdateActionRequestApproveInfo,
		entity.TaskUpdateActionSLABreachRequestAddInfo,
		entity.TaskUpdateActionDayBeforeSLARequestAddInfo,
	}, in.Action)

	// nolint:gocritic,goconst
	if in.StepType == "form" {
		notInStatuses = []string{"skipped"}
	} else if (in.StepType == "execution" || in.StepType == "approver") && isAddInfoReq {
		notInStatuses = []string{"skipped"}
	} else {
		notInStatuses = []string{"skipped", "finished"}
	}

	args := []interface{}{in.ID, in.StepType, notInStatuses}

	var stepNamesQ string

	if len(in.StepNames) > 0 {
		stepNamesQ = "vs.step_name = ANY($4) AND"

		args = append(args, in.StepNames)
	}

	// nolint:gocritic
	// language=PostgreSQL
	q := fmt.Sprintf(`
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
	    vs.is_paused = false AND
	    work_id = $1 AND 
	    step_type = $2 AND 
	    NOT status = ANY($3) AND 
		%s
	    vs.time = (SELECT max(time) FROM variable_storage WHERE work_id = $1 AND step_name = vs.step_name)
	    ORDER BY vs.time ASC`,
		stepNamesQ,
	)

	rows, err := db.Connection.Query(ctx, q, args...)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return []*entity.Step{}, nil
		}

		return nil, err
	}

	defer rows.Close()

	//nolint:dupl //scan
	for rows.Next() {
		var (
			s       entity.Step
			content string
		)

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

	defer rows.Close()

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

func (db *PGCon) UnsetIsActive(ctx context.Context, workNumber, blockName string) error {
	ctx, span := trace.StartSpan(ctx, "pg_unset_is_active")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	const q = `WITH RECURSIVE all_nodes AS(
    	SELECT distinct key(jsonb_each(v.content #> '{pipeline,blocks}'))::text out_node,
                    jsonb_array_elements_text(value(jsonb_each(value(jsonb_each(v.content #> '{pipeline,blocks}'))->'next'))) AS in_node
    	FROM works w
             INNER JOIN versions v ON w.version_id=v.id
    			WHERE w.work_number=$1
		),
        next_gates_nodes AS(
                SELECT out_node,
                          in_node,
                          1 AS level,
                          Array[out_node] AS circle_check
                FROM all_nodes
                WHERE out_node=$2
                UNION ALL
                SELECT a.out_node,
                          a.in_node,
                          CASE WHEN a.out_node LIKE 'begin_parallel_task%' THEN ign.level+1
                               WHEN a.out_node LIKE 'wait_for_all_inputs%' THEN ign.level-1
                               ELSE ign.level end AS level,
                          array_append(ign.circle_check, a.out_node)
                FROM all_nodes a
                INNER JOIN next_gates_nodes ign ON ign.in_node=a.out_node
                WHERE array_position(circle_check,a.in_node) IS null
               )
		UPDATE variable_storage AS v
			SET is_active=false
		FROM variable_storage vs
		INNER JOIN works w ON vs.work_id = w.id
		WHERE v.id=vs.id AND w.work_number=$1 AND w.child_id IS null AND vs.step_name IN (SELECT distinct in_node
                                                                                  FROM next_gates_nodes
                                                                                  WHERE level>0);
	`

	_, err := db.Connection.Exec(ctx, q, workNumber, blockName)

	return err
}

func (db *PGCon) ParallelIsFinished(ctx context.Context, workNumber, blockName string) (bool, error) {
	ctx, span := trace.StartSpan(ctx, "pg_parallel_is_finished")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	const q = `WITH RECURSIVE all_nodes AS(
		SELECT distinct key(jsonb_each(v.content #> '{pipeline,blocks}'))::text out_node,
			jsonb_array_elements_text(value(jsonb_each(value(jsonb_each(v.content #> '{pipeline,blocks}'))->'next'))) AS in_node
		FROM works w
		INNER JOIN versions v ON w.version_id=v.id
		WHERE w.work_number=$1
	),
	inside_gates_nodes AS(
	   SELECT in_node,
			  out_node,
			  1 AS level,
			  Array[in_node] AS circle_check
	   FROM all_nodes
	   WHERE in_node=$2
	   UNION ALL
	   SELECT a.in_node,
			  a.out_node,
			  CASE WHEN a.out_node LIKE 'wait_for_all_inputs%' THEN ign.level+1
				   WHEN a.out_node LIKE 'begin_parallel_task%' THEN ign.level-1
				   else ign.level end AS level,
			  array_append(ign.circle_check, a.in_node)
	   FROM all_nodes a
				INNER JOIN inside_gates_nodes ign ON a.in_node=ign.out_node
	   WHERE array_position(circle_check,a.out_node) is null AND
			   a.in_node NOT LIKE 'begin_parallel_task%' AND ign.level!=0)
	SELECT
    (
        SELECT CASE WHEN count(*)=0 THEN true ELSE false end
        FROM variable_storage vs
                 INNER JOIN works w on vs.work_id = w.id
                 INNER JOIN inside_gates_nodes ign ON vs.step_name=ign.out_node
        WHERE w.work_number=$1 and w.child_id is null and vs.status IN('running', 'idle', 'ready') 
          AND is_active = true
    ) as is_finished,
    (
        SELECT CASE WHEN count(distinct vs.step_name) = 
			(SELECT count(distinct inside_gates_nodes.in_node) 
				FROM inside_gates_nodes 
			WHERE out_node LIKE 'begin_parallel_task_%')
        THEN true ELSE false end
    	FROM variable_storage vs
                 INNER JOIN works w ON vs.work_id = w.id
                 INNER JOIN inside_gates_nodes ign ON vs.step_name=ign.in_node
        WHERE w.work_number=$1 and w.child_id is null AND ign.out_node LIKE 'begin_parallel_task_%' AND is_active = true
	) AS created_all_branches`

	var parallelIsFinished, createdAllBranches bool

	row := db.Connection.QueryRow(ctx, q, workNumber, blockName)

	if err := row.Scan(&parallelIsFinished, &createdAllBranches); err != nil {
		return false, err
	}

	return parallelIsFinished && createdAllBranches, nil
}

func (db *PGCon) GetTaskStepByID(ctx context.Context, id uuid.UUID) (*entity.Step, error) {
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
		w.run_context -> 'initial_application' -> 'is_test_application' as isTest,
		vs.is_paused
	FROM variable_storage vs 
	JOIN works w ON vs.work_id = w.id
		WHERE vs.id = $1
	LIMIT 1`

	var (
		s       entity.Step
		content string
	)

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
		&s.IsPaused,
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
	workID uuid.UUID, stepName string,
) (*entity.Step, error) {
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

	var (
		s       entity.Step
		content string
	)

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
	ctx, span := trace.StartSpan(ctx, "pg_get_canceled_task_steps")
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
			vs.status,
			vs.is_paused
		FROM variable_storage vs  
			WHERE vs.work_id = $1 AND vs.step_name = $2
			ORDER BY vs.time DESC
		LIMIT 1
`

	var (
		s       entity.Step
		content string
	)

	err := db.Connection.QueryRow(ctx, query, workID, stepName).Scan(
		&s.ID,
		&s.Type,
		&s.Name,
		&s.Time,
		&content,
		&s.BreakPoints,
		&s.HasError,
		&s.Status,
		&s.IsPaused,
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
	res.PipelineID = pID
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
       vs.raw_start_schema,
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
		rss      []byte
		a        string
	)

	err := db.Connection.QueryRow(c, query, pipelineID).Scan(&vID, &s, &pID, &ca, &content, &cr, &cm, &a, &ss, &es, &rss, &d)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal([]byte(content), res)
	if err != nil {
		return nil, err
	}

	res.VersionID = vID
	res.PipelineID = pID
	res.Status = s
	res.CommentRejected = cr
	res.Comment = cm
	res.ApprovedAt = d
	res.CreatedAt = ca
	res.Settings.StartSchema = ss
	res.Settings.EndSchema = es
	res.Settings.StartSchemaRaw = rss
	res.Author = a

	return res, nil
}

func (db *PGCon) GetPipelinesByNameOrID(ctx context.Context, dto *SearchPipelineRequest) ([]entity.SearchPipeline, error) {
	c, span := trace.StartSpan(ctx, "pg_get_pipelines_by_name_or_id")
	defer span.End()

	res := make([]entity.SearchPipeline, 0, dto.Limit)

	// nolint:gocritic
	// language=PostgreSQL
	q := `
		SELECT
			p.id,
			p.name,
			count(*) over () as total
		FROM pipelines p
		WHERE p.deleted_at IS NULL  --pipe--
		LIMIT $1 OFFSET $2;
`

	if dto.PipelineName != nil {
		pipelineName := strings.ReplaceAll(*dto.PipelineName, "_", "!_")
		pipelineName = strings.ReplaceAll(pipelineName, "%", "!%")
		q = strings.ReplaceAll(q, "--pipe--", fmt.Sprintf("AND p.name ilike'%%%s%%' ESCAPE '!'", pipelineName))
	}

	if dto.PipelineID != nil {
		q = strings.ReplaceAll(q, "--pipe--", fmt.Sprintf("AND p.id='%s'", *dto.PipelineID))
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
			&s.PipelineID,
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
	q := `
			with accesses as (
    select jsonb_array_elements(content -> 'State' -> step_name -> 'forms_accessibility') as data
    from variable_storage
    where ((step_type = 'approver' and content -> 'State' -> step_name -> 'approvers' ? $3)
        or (step_type = 'execution' and content -> 'State' -> step_name -> 'executors' ? $3)
        or (step_type = 'form' and content -> 'State' -> step_name -> 'executors' ? $3)
		or (step_type = 'sign' and content -> 'State' -> step_name -> 'signers' ? $3))
      and status in ('idle', 'running')
      and work_id = (SELECT id
                     FROM works
                     WHERE work_number = $1
                       AND child_id IS NULL
    )
)
select count(*)
from accesses
where accesses.data::jsonb ->> 'node_id' = $2
  and accesses.data::jsonb ->> 'accessType' in ('ReadWrite', 'RequiredFill')
`

	var count int

	if scanErr := db.Connection.QueryRow(ctx, q, workNumber, stepName, login).Scan(&count); scanErr != nil {
		return false, scanErr
	}

	return count != 0, nil
}

func (db *PGCon) GetAdditionalDescriptionForms(workNumber, nodeName string) ([]entity.DescriptionForm, error) {
	const query = `
	WITH content as (
		SELECT jsonb_array_elements(content -> 'pipeline' -> 'blocks' -> $2 -> 'params' -> 'forms_accessibility') as rules
		FROM versions
			WHERE id = (SELECT version_id FROM works WHERE work_number = $1 AND child_id IS NULL)

		UNION

		SELECT jsonb_array_elements(content -> 'pipeline' -> 'blocks' -> $2 -> 'params' -> 'formsAccessibility') as rules
		FROM versions
			WHERE id = (SELECT version_id FROM works WHERE work_number = $1 AND child_id IS NULL)
	)
    SELECT v.content -> 'State' -> v.step_name -> 'application_body', v.step_name
	FROM variable_storage v
	    INNER JOIN  (
		      SELECT max(time) as mtime, step_name from variable_storage
	          where work_id = (SELECT id FROM works WHERE work_number = $1 AND child_id IS NULL)
		      group by step_name
        ) t ON t.mtime= v.time and t.step_name=v.step_name
		WHERE v.step_name in (
			SELECT rules ->> 'node_id' as rule
			FROM content
			WHERE rules ->> 'accessType' != 'None'
		)
		AND v.work_id = (SELECT id FROM works WHERE work_number = $1 AND child_id IS NULL)
	ORDER BY v.time`

	descriptionForms := make([]entity.DescriptionForm, 0)

	rows, err := db.Connection.Query(context.Background(), query, workNumber, nodeName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return descriptionForms, nil
		}

		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var (
			formName string
			form     orderedmap.OrderedMap
		)

		if scanErr := rows.Scan(&form, &formName); scanErr != nil {
			return descriptionForms, scanErr
		}

		descriptionForms = append(descriptionForms, entity.DescriptionForm{
			Name:        formName,
			Description: form,
		})
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		return descriptionForms, rowsErr
	}

	return descriptionForms, nil
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

func (db *PGCon) GetBlockDataFromVersion(ctx context.Context, workNumber, stepName string) (*entity.EriusFunc, error) {
	ctx, span := trace.StartSpan(ctx, "get_block_data_from_version")
	defer span.End()

	q := `
		SELECT content->'pipeline'->'blocks'->$1 FROM versions
    	JOIN works w ON versions.id = w.version_id
		WHERE w.work_number = $2 AND w.child_id IS NULL`

	var f *entity.EriusFunc

	if scanErr := db.Connection.QueryRow(ctx, q, stepName, workNumber).Scan(&f); scanErr != nil {
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

func (db *PGCon) FinishTaskBlocks(ctx context.Context, taskID uuid.UUID, ignoreSteps []string, updateParent bool) (err error) {
	ctx, span := trace.StartSpan(ctx, "finish_task_blocks")
	defer span.End()

	var filter string
	if updateParent {
		filter = "(SELECT id FROM works WHERE child_id = $1)"
	} else {
		filter = "$1"
	}

	q := fmt.Sprintf(`
		UPDATE variable_storage
		SET status = 'finished', updated_at = now()
		WHERE work_id = %s AND status IN ('ready', 'idle', 'running')`, filter)

	if len(ignoreSteps) > 0 {
		q += " AND step_name NOT IN ('" + strings.Join(ignoreSteps, "','") + "')"
	}

	_, err = db.Connection.Exec(ctx, q, taskID)

	return err
}

func (db *PGCon) GetVariableStorageForStep(ctx context.Context, taskID uuid.UUID, stepName string) (*store.VariableStore, error) {
	ctx, span := trace.StartSpan(ctx, "get_variable_storage_for_step")
	defer span.End()

	const q = `
		SELECT content
		FROM variable_storage
		WHERE work_id = $1 AND step_name = $2 
		ORDER BY time DESC LIMIT 1`

	var content []byte

	if err := db.Connection.QueryRow(ctx, q, taskID, stepName).Scan(&content); err != nil {
		return nil, err
	}

	storage := store.NewStore()

	if err := json.Unmarshal(content, &storage); err != nil {
		return nil, err
	}

	return storage, nil
}

func (db *PGCon) GetVariableStorage(ctx context.Context, workNumber string) (*store.VariableStore, error) {
	ctx, span := trace.StartSpan(ctx, "get_variable_storage")
	defer span.End()

	const q = `
		SELECT step_name, vs.content
		FROM variable_storage vs 
			WHERE vs.work_id = (SELECT id FROM works 
			                 	WHERE work_number = $1 AND child_id IS NULL LIMIT 1) AND 
			vs.time = (SELECT max(time) FROM variable_storage WHERE work_id = vs.work_id AND step_name = vs.step_name)`

	states := make([]map[string]interface{}, 0)

	rows, err := db.Connection.Query(ctx, q, workNumber)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var (
			stepName string
			content  []byte
		)

		if scanErr := rows.Scan(&stepName, &content); scanErr != nil {
			return nil, scanErr
		}

		storage := store.NewStore()
		if unmErr := json.Unmarshal(content, &storage); unmErr != nil {
			return nil, unmErr
		}

		storage.Values[stepNameVariable] = stepName

		states = append(states, storage.Values)
	}

	err = rows.Err()
	if err != nil {
		return nil, err
	}

	storage := &store.VariableStore{
		Values: mergeValues(states),
	}

	return storage, nil
}

const stepNameVariable = "stepName"

func mergeValues(stepsValues []map[string]interface{}) map[string]interface{} {
	res := make(map[string]interface{})

	for i := range stepsValues {
		stepName, ok := stepsValues[i][stepNameVariable]
		if !ok {
			continue
		}

		prefix := fmt.Sprintf("%s.", stepName)

		for varName := range stepsValues[i] {
			if _, exists := res[varName]; !exists &&
				varName != stepNameVariable &&
				strings.HasPrefix(varName, prefix) {
				res[varName] = stepsValues[i][varName]
			}
		}
	}

	return res
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
			   w.run_context -> 'initial_application' -> 'is_test_application' as isTest,
			   CASE 
			    	WHEN w.run_context -> 'initial_application' -> 'custom_title' IS NULL
			        	THEN ''
			        	ELSE w.run_context -> 'initial_application' ->> 'custom_title'
				END as customTitle
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
			AND d.deadline < NOW()
			AND vs.is_paused = false`

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
			&item.CustomTitle,
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

func (db *PGCon) GetTaskActiveBlock(c context.Context, taskID, stepName string) ([]string, error) {
	c, span := trace.StartSpan(c, "pg_get_task_active_block")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := `SELECT step_name 
			FROM variable_storage 
			WHERE 
			    status IN ('running', 'idle') 
				AND step_type IN ('approver', 'sign', 'form', 'execution') AND work_id = $1 and step_name != $2`

	stepNames := make([]string, 0)

	rows, err := db.Connection.Query(c, q, taskID, stepName)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var name string

		if scanErr := rows.Scan(&name); scanErr != nil {
			return stepNames, scanErr
		}

		stepNames = append(stepNames, name)
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}

	return stepNames, nil
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
	systemID string,
) (entity.ExternalSystemSubscriptionParams, error) {
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
	systemID string,
) (entity.ExternalSystemSubscriptionParams, error) {
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

func (db *PGCon) SaveExternalSystemSubscriptionParams(ctx context.Context, versionID string,
	params *entity.ExternalSystemSubscriptionParams,
) error {
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
	excludeWorkNumber string,
) ([]*entity.EriusTask, error) {
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

	var finishedAt sql.NullTime

	err := row.Scan(
		&interval.StartedAt,
		&finishedAt,
	)
	if err != nil {
		return &entity.TaskCompletionInterval{}, err
	}

	if finishedAt.Valid {
		interval.FinishedAt = finishedAt.Time
	}

	return &interval, nil
}

func (db *PGCon) GetVersionsByFunction(ctx context.Context, funcID, versionID string) ([]entity.EriusScenario, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_versions_by_function")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	const q = `
	SELECT p.name, v.id, p.author, v.status
    FROM versions v
	JOIN pipelines p on v.pipeline_id = p.id
    JOIN LATERAL jsonb_each(v.content->'pipeline'->'blocks') as bks on true
    JOIN LATERAL (
        SELECT 
        	pipeline_id,
        	max(created_at) AS max_version_date
        FROM versions
        GROUP BY pipeline_id
    ) latest ON latest.pipeline_id = v.pipeline_id
	WHERE (
		(v.status = 2 AND v.is_actual = true) OR
	    (v.status = 1 AND v.created_at = latest.max_version_date)
	)
	AND v.deleted_at IS NULL
	AND bks.value ->> 'type_id' = 'executable_function'
    AND bks.value->'params'->'function'->>'functionId' = $1
    AND bks.value->'params'->'function'->>'versionId' != $2
    GROUP BY p.name, v.id, p.author;
`

	rows, err := db.Connection.Query(ctx, q, funcID, versionID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	versions := make([]entity.EriusScenario, 0)

	for rows.Next() {
		v := entity.EriusScenario{}

		err = rows.Scan(&v.Name, &v.VersionID, &v.Author, &v.Status)
		if err != nil {
			return nil, err
		}

		versions = append(versions, v)
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}

	return versions, nil
}

func (db *PGCon) GetTaskStepByNameForCtxEditing(ctx context.Context, workID uuid.UUID, stepName string, t time.Time) (*entity.Step, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_task_step_by_name")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	const query = `
		SELECT 
			vs.id,
			vs.step_type,
			vs.step_name, 
		FROM variable_storage vs  
			WHERE vs.work_id = $1 AND vs.step_name = $2 AND time < $3
			ORDER BY vs.time DESC
		LIMIT 1
`

	var s entity.Step

	err := db.Connection.QueryRow(ctx, query, workID, stepName, t).Scan(
		&s.ID,
		&s.Type,
		&s.Name,
	)
	if err != nil {
		return nil, err
	}

	return &s, nil
}

func (db *PGCon) SaveNodePreviousContent(ctx context.Context, stepID, eventID string) error {
	ctx, span := trace.StartSpan(ctx, "pg_save_node_previous_content")
	defer span.End()

	id := uuid.New()

	// nolint:gocritic
	// language=PostgreSQL
	q := `
	INSERT INTO edit_nodes_history (
		id,
		event_id,
		step_id,
		content                                                           
	)
	VALUES (
		$1, 
		$2, 
		$3, 
		(
		    SELECT content FROM variable_storage 
		                   WHERE id = $3
		)
	)`

	_, err := db.Connection.Exec(ctx, q, id, eventID, stepID)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGCon) UpdateNodeContent(ctx context.Context, stepID, workID,
	stepName string, state, output map[string]interface{},
) error {
	ctx, span := trace.StartSpan(ctx, "pg_update_node_content")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	qState := `
		WITH blockStartTime AS (
		    SELECT time FROM variable_storage WHERE id = $1                                    
		),
		endTime AS (
		        SELECT time FROM variable_storage 
      			WHERE work_id = $4 AND step_name = $2 AND time > (SELECT time FROM blockStartTime)
       			ORDER BY time DESC
				LIMIT 1  
		    )
	    UPDATE variable_storage
        SET content = jsonb_set(content, array['State', $2]::varchar[], $3::jsonb , false)
    		WHERE work_id = $4 AND time >= (SELECT time FROM blockStartTime)
      		AND CASE 
      		    WHEN (SELECT time FROM endTime) IS NOT NULL THEN time < (SELECT time FROM endTime)
				ELSE TRUE
				END
    `
	stateArgs := []interface{}{
		stepID,
		stepName,
		state,
		workID,
	}

	_, stateErr := db.Connection.Exec(ctx, qState, stateArgs...)
	if stateErr != nil {
		return stateErr
	}

	for key, val := range output {
		// nolint:gocritic
		// language=PostgreSQL
		qOutput := `
		WITH blockStartTime AS (
		    SELECT time FROM variable_storage WHERE id = $1                                    
		),
		endTime AS (
		        SELECT time from variable_storage 
      			WHERE work_id = $4 AND step_name = $5 AND time > (SELECT time FROM blockStartTime)
       			ORDER BY time DESC
				LIMIT 1  
		    )
	    UPDATE variable_storage
        SET content = jsonb_set(content, array['Values', $2]::varchar[], $3::jsonb , false)
    		WHERE work_id = $4 AND time >= (SELECT time FROM blockStartTime)
      		AND CASE 
      		    WHEN (SELECT time FROM endTime) IS NOT NULL THEN time < (SELECT time FROM endTime)
				ELSE TRUE
				END
    `

		dbVal := wrapVal(val)

		outputArgs := []interface{}{
			stepID,
			stepName + "." + key,
			dbVal,
			workID,
			stepName,
		}

		_, outPutErr := db.Connection.Exec(ctx, qOutput, outputArgs...)
		if outPutErr != nil {
			return outPutErr
		}
	}

	return nil
}

func wrapVal(data interface{}) interface{} {
	convData, ok := data.(string)
	if !ok {
		return data
	}

	return fmt.Sprintf("%q", convData)
}
