package db

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/jackc/pgconn"

	"github.com/jackc/pgx/v4"

	"go.opencensus.io/trace"

	e "gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

func (db *PGCon) copyProcessSettingsFromOldVersion(c context.Context, newVersionID, oldVersionID uuid.UUID) error {
	const qCopyPrevSettings = `
	INSERT INTO version_settings (id, version_id, start_schema, end_schema, raw_start_schema, resubmission_period) 
		SELECT uuid_generate_v4(), $1, start_schema, end_schema, raw_start_schema, resubmission_period
		FROM version_settings 
		WHERE version_id = $2`

	_, err := db.Connection.Exec(c, qCopyPrevSettings, newVersionID, oldVersionID)
	if err != nil {
		return err
	}

	const qCopyExternalSystems = `
	INSERT INTO external_systems (
		id,
		version_id,
		system_id,
		input_schema,
		output_schema,
		input_mapping,
		output_mapping,
		microservice_id,
		ending_url,
		sending_method,
		allow_run_as_others)
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
	WHERE version_id = $2;`

	_, err = db.Connection.Exec(c, qCopyExternalSystems, newVersionID, oldVersionID)
	if err != nil {
		return err
	}

	// nolint:gocritic
	// language=PostgreSQL
	const qCopyPrevSLASettings = `
	INSERT INTO version_sla (id, version_id, author,created_at,work_type,sla) 
		SELECT uuid_generate_v4(), $1, author, now(), work_type, sla
			FROM version_sla 
		WHERE version_id = $2
		ORDER BY created_at DESC LIMIT 1;
	`

	_, err = db.Connection.Exec(c, qCopyPrevSLASettings, newVersionID, oldVersionID)
	if err != nil {
		return err
	}

	// nolint:gocritic
	// language=PostgreSQL
	const qCopyPrevTaskSubSettings = `
		INSERT INTO external_system_task_subscriptions (
			id,
			version_id,
			system_id,
			microservice_id,
			path, 
			method,
			notification_schema,
			mapping,
			nodes)
		SELECT uuid_generate_v4(), $1, system_id, microservice_id, path, method, notification_schema, mapping, nodes 
			FROM external_system_task_subscriptions
		WHERE version_id = $2`

	_, err = db.Connection.Exec(c, qCopyPrevTaskSubSettings, newVersionID, oldVersionID)
	if err != nil {
		return err
	}

	const qCopyPrevApprovalLists = `
	INSERT INTO version_approval_lists (
		id,
		version_id,
		name,
		steps,
		context_mapping,
		forms_mapping,
		created_at) 
	SELECT 
		uuid_generate_v4(), 
		$1,
		name,
		steps,
		context_mapping,
		forms_mapping,
		now()
	FROM version_approval_lists 
	WHERE version_id = $2 AND deleted_at IS NULL`

	_, err = db.Connection.Exec(c, qCopyPrevApprovalLists, newVersionID, oldVersionID)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGCon) GetVersionSettings(ctx context.Context, versionID string) (e.ProcessSettings, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_version_settings")
	defer span.End()

	// nolint:gocritic,lll
	// language=PostgreSQL
	const query = `
	SELECT start_schema, end_schema, resubmission_period, raw_start_schema,
	    (select p.name from pipelines p where p.id = 
	       (select pipeline_id from versions v where v.id = 
	       	(select version_id from version_settings vs where vs.id = version_settings.id)
	       )
	    ) "name"
	FROM version_settings
	WHERE version_id = $1`

	row := db.Connection.QueryRow(ctx, query, versionID)

	ps := e.ProcessSettings{VersionID: versionID}

	err := row.Scan(&ps.StartSchema, &ps.EndSchema, &ps.ResubmissionPeriod, &ps.StartSchemaRaw, &ps.Name)
	if err != nil && err != pgx.ErrNoRows {
		return ps, err
	}

	return ps, nil
}

//nolint:gocritic //нужно для реализации интерфейса Database
func (db *PGCon) SaveVersionSettings(ctx context.Context, settings e.ProcessSettings, schemaFlag *string) error {
	ctx, span := trace.StartSpan(ctx, "pg_save_version_settings")
	defer span.End()

	var (
		err        error
		commandTag pgconn.CommandTag
	)

	if schemaFlag == nil {
		// nolint:gocritic
		// language=PostgreSQL
		query := `
		INSERT INTO version_settings (id, version_id, start_schema, end_schema, raw_start_schema) 
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (version_id) DO UPDATE 
			SET start_schema = excluded.start_schema, 
				end_schema = excluded.end_schema,
				raw_start_schema = excluded.raw_start_schema`
		commandTag, err = db.Connection.Exec(ctx,
			query,
			uuid.New(),
			settings.VersionID,
			settings.StartSchema,
			settings.EndSchema,
			settings.StartSchemaRaw,
		)
		if err != nil {
			return err
		}
	} else {
		switch *schemaFlag {
		case startSchema:
			// nolint:gocritic
			// language=PostgreSQL
			query := `INSERT INTO version_settings (id,version_id,start_schema, raw_start_schema) 
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (version_id) DO UPDATE 
				SET start_schema = excluded.start_schema,
			    raw_start_schema = excluded.raw_start_schema`

			commandTag, err = db.Connection.Exec(ctx, query, uuid.New(), settings.VersionID, settings.StartSchema, settings.StartSchemaRaw)
			if err != nil {
				return err
			}
		case endSchema:
			// nolint:gocritic
			// language=PostgreSQL
			query := `INSERT INTO version_settings (id, version_id, end_schema) 
			VALUES ($1, $2, $3)
			ON CONFLICT (version_id) DO UPDATE 
				SET end_schema = excluded.end_schema`

			commandTag, err = db.Connection.Exec(ctx, query, uuid.New(), settings.VersionID, settings.EndSchema)
			if err != nil {
				return err
			}
		default:
			return errUnkonwnSchemaFlag
		}
	}

	if commandTag.RowsAffected() != 0 {
		_ = db.RemoveObsoleteMapping(ctx, settings.VersionID)
	}

	return nil
}

//nolint:gocritic //нужно для реализации интерфейса Database
func (db *PGCon) SaveVersionMainSettings(ctx context.Context, params e.ProcessSettings) error {
	ctx, span := trace.StartSpan(ctx, "pg_save_version_main_settings")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	const query = `INSERT INTO version_settings (id, version_id, resubmission_period) 
			VALUES ($1, $2, $3)
			ON CONFLICT (version_id) DO UPDATE 
			SET resubmission_period = excluded.resubmission_period`

	_, err := db.Connection.Exec(ctx, query, uuid.New(), params.VersionID, params.ResubmissionPeriod)
	if err != nil {
		return err
	}

	return nil
}

//nolint:gocritic //всегда передавалось по значению
func (db *PGCon) SaveExternalSystemSettings(ctx context.Context, vID string, system e.ExternalSystem, schemaFlag *string) error {
	ctx, span := trace.StartSpan(ctx, "pg_save_external_system_settings")
	defer span.End()

	args := []interface{}{vID, system.ID}

	thrisArgPostFix := fmt.Sprintf(" = $%d", len(args)+1)

	var schemasForUpdate string

	if schemaFlag != nil {
		switch *schemaFlag {
		case inputSchema:
			schemasForUpdate = inputSchema + thrisArgPostFix

			args = append(args, system.InputSchema)
		case outputSchema:
			schemasForUpdate = outputSchema + thrisArgPostFix

			args = append(args, system.OutputSchema)
		case inputMapping:
			schemasForUpdate = inputMapping + thrisArgPostFix

			args = append(args, system.InputMapping)
		case outputMapping:
			schemasForUpdate = outputMapping + thrisArgPostFix

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

func (db *PGCon) GetExternalSystemSettings(ctx context.Context, versionID, systemID string) (e.ExternalSystem, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_external_system_settings")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	const query = `
	SELECT input_schema, output_schema, input_mapping, output_mapping,
	microservice_id, ending_url, sending_method, allow_run_as_others
		FROM external_systems
	WHERE version_id = $1 AND system_id = $2`

	row := db.Connection.QueryRow(ctx, query, versionID, systemID)

	res := e.ExternalSystem{ID: systemID, OutputSettings: &e.EndSystemSettings{}}

	err := row.Scan(
		&res.InputSchema,
		&res.OutputSchema,
		&res.InputMapping,
		&res.OutputMapping,
		&res.OutputSettings.MicroserviceID,
		&res.OutputSettings.URL,
		&res.OutputSettings.Method,
		&res.AllowRunAsOthers,
	)
	if err != nil {
		return res, err
	}

	return res, nil
}

func (db *PGCon) UpdateEndingSystemSettings(ctx context.Context, versionID, systemID string, s e.EndSystemSettings) (err error) {
	ctx, span := trace.StartSpan(ctx, "pg_update_ending_system_settings")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	const query = `
	UPDATE external_systems
		SET (microservice_id, ending_url, sending_method) = ($1, $2, $3)
	WHERE version_id = $4 AND system_id = $5`

	_, err = db.Connection.Exec(ctx, query, s.MicroserviceID, s.URL, s.Method, versionID, systemID)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGCon) SaveSLAVersionSettings(ctx context.Context, versionID string, s e.SLAVersionSettings) (err error) {
	ctx, span := trace.StartSpan(ctx, "pg_save_sla_version_settings")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	const query = `
	INSERT INTO version_sla (id, version_id, author, created_at, work_type, sla)
	VALUES ( $1, $2, $3, now(), $4, $5)`

	_, err = db.Connection.Exec(ctx, query, uuid.New(), versionID, s.Author, s.WorkType, s.SLA)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGCon) GetSLAVersionSettings(ctx context.Context, versionID string) (s e.SLAVersionSettings, err error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_sla_version_settings")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	const query = `
	SELECT author, work_type, sla
		FROM version_sla
	WHERE version_id = $1
	ORDER BY created_at DESC`

	row := db.Connection.QueryRow(ctx, query, versionID)

	slaSettings := e.SLAVersionSettings{}

	err = row.Scan(&slaSettings.Author, &slaSettings.WorkType, &slaSettings.SLA)
	if err != nil {
		return e.SLAVersionSettings{}, err
	}

	return slaSettings, nil
}

func (db *PGCon) GetApprovalListSettings(ctx context.Context, listID string) (*e.ApprovalListSettings, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_approval_list_settings")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	const query = `
	SELECT id, name, steps, context_mapping, forms_mapping, created_at
		FROM version_approval_lists
	WHERE id = $1`

	row := db.Connection.QueryRow(ctx, query, listID)

	res := e.ApprovalListSettings{}

	err := row.Scan(
		&res.ID,
		&res.Name,
		&res.Steps,
		&res.ContextMapping,
		&res.FormsMapping,
		&res.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &res, nil
}

func (db *PGCon) GetApprovalListsSettings(ctx context.Context, versionID string) ([]e.ApprovalListSettings, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_approval_lists_settings")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	const query = `
	SELECT id, name, steps, context_mapping, forms_mapping, created_at
		FROM version_approval_lists
	WHERE version_id = $1 AND deleted_at IS NULL
	ORDER BY created_at DESC`

	rows, err := db.Connection.Query(ctx, query, versionID)
	if err != nil {
		return nil, err
	}

	res := make([]e.ApprovalListSettings, 0)

	for rows.Next() {
		al := e.ApprovalListSettings{}

		err = rows.Scan(
			&al.ID,
			&al.Name,
			&al.Steps,
			&al.ContextMapping,
			&al.FormsMapping,
			&al.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		res = append(res, al)
	}

	return res, nil
}

func (db *PGCon) SaveApprovalListSettings(ctx context.Context, in e.SaveApprovalListSettings) (id string, err error) {
	ctx, span := trace.StartSpan(ctx, "pg_save_approval_list_settings")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	const query = `
	INSERT INTO version_approval_lists (
		id,
		version_id,
		name,
		steps,
		context_mapping,
		forms_mapping,
		created_at)
	VALUES ($1, $2, $3, $4, $5, $6, now())`

	listID := uuid.New().String()

	_, err = db.Connection.Exec(
		ctx,
		query,
		listID,
		in.VersionID,
		in.Name,
		in.Steps,
		in.ContextMapping,
		in.FormsMapping)
	if err != nil {
		return "", err
	}

	return listID, nil
}

func (db *PGCon) UpdateApprovalListSettings(ctx context.Context, in e.UpdateApprovalListSettings) error {
	ctx, span := trace.StartSpan(ctx, "pg_update_approval_list_settings")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	const query = `
	UPDATE version_approval_lists
		SET (name, steps, context_mapping, forms_mapping) = ($1, $2, $3, $4)
	WHERE id = $5`

	_, err := db.Connection.Exec(
		ctx,
		query,
		in.Name,
		in.Steps,
		in.ContextMapping,
		in.FormsMapping,
		in.ID)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGCon) RemoveApprovalListSettings(ctx context.Context, listID string) error {
	ctx, span := trace.StartSpan(ctx, "pg_remove_approval_list_settings")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	const query = `
	UPDATE version_approval_lists
		SET deleted_at = now()
	WHERE id = $1`

	_, err := db.Connection.Exec(ctx, query, listID)
	if err != nil {
		return err
	}

	return nil
}
