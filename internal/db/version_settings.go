package db

import (
	c "context"
	"fmt"
	"golang.org/x/net/context"

	"github.com/google/uuid"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"

	e "gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"

	"go.opencensus.io/trace"
)

func (db *PGCon) copyProcessSettingsFromOldVersion(c c.Context, newVersionID, oldVersionID uuid.UUID) error {
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
	const qCopyPrevSlaSettings = `
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

	return nil
}

func (db *PGCon) GetVersionSettings(ctx c.Context, versionID string) (e.ProcessSettings, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_version_settings")
	defer span.End()

	// nolint:gocritic,lll
	// language=PostgreSQL
	const query = `
	SELECT start_schema, end_schema, resubmission_period,
	    (select p.name from pipelines p where p.id = 
	       (select pipeline_id from versions v where v.id = 
	       	(select version_id from version_settings vs where vs.id = version_settings.id)
	       )
	    ) "name"
	FROM version_settings
	WHERE version_id = $1`

	row := db.Connection.QueryRow(ctx, query, versionID)

	ps := e.ProcessSettings{Id: versionID}
	err := row.Scan(&ps.StartSchema, &ps.EndSchema, &ps.ResubmissionPeriod, &ps.Name)
	if err != nil && err != pgx.ErrNoRows {
		return ps, err
	}

	return ps, nil
}

func (db *PGCon) SaveVersionSettings(ctx c.Context, settings e.ProcessSettings, schemaFlag *string) error {
	ctx, span := trace.StartSpan(ctx, "pg_save_version_settings")
	defer span.End()

	var err error
	var commandTag pgconn.CommandTag

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

func (db *PGCon) SaveVersionMainSettings(ctx c.Context, params e.ProcessSettings) error {
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

func (db *PGCon) SaveExternalSystemSettings(ctx c.Context, vID string, system e.ExternalSystem, schemaFlag *string) error {
	ctx, span := trace.StartSpan(ctx, "pg_save_external_system_settings")
	defer span.End()

	args := []interface{}{vID, system.Id}
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

	externalSystemSettings := e.ExternalSystem{Id: systemID, OutputSettings: &e.EndSystemSettings{}}
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

func (db *PGCon) UpdateEndingSystemSettings(ctx context.Context, versionID, systemID string, s e.EndSystemSettings) (err error) {
	ctx, span := trace.StartSpan(ctx, "pg_update_ending_system_settings")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	const query = `
	UPDATE external_systems
		SET (microservice_id, ending_url, sending_method) = ($1, $2, $3)
	WHERE version_id = $4 AND system_id = $5`

	_, err = db.Connection.Exec(ctx, query, s.MicroserviceId, s.URL, s.Method, versionID, systemID)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGCon) SaveSlaVersionSettings(ctx context.Context, versionID string, s e.SlaVersionSettings) (err error) {
	ctx, span := trace.StartSpan(ctx, "pg_save_sla_version_settings")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	const query = `
	INSERT INTO version_sla (id, version_id, author, created_at, work_type, sla)
	VALUES ( $1, $2, $3, now(), $4, $5)`

	_, err = db.Connection.Exec(ctx, query, uuid.New(), versionID, s.Author, s.WorkType, s.Sla)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGCon) GetSlaVersionSettings(ctx context.Context, versionID string) (s e.SlaVersionSettings, err error) {
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
	slaSettings := e.SlaVersionSettings{}
	if err = row.Scan(&slaSettings.Author, &slaSettings.WorkType, &slaSettings.Sla); err != nil {
		return e.SlaVersionSettings{}, err
	}
	return slaSettings, nil
}

func (db *PGCon) GetApprovalListSettings(ctx c.Context, versionID, listID string) (*e.ApprovalListSettings, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_approval_list_settings")
	defer span.End()

	return nil, nil
}

func (db *PGCon) GetApprovalListsSettings(ctx c.Context, versionID string) ([]e.ApprovalListSettings, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_approval_lists_settings")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	const query = `
	SELECT id, name, steps, context_mapping, forms_mapping, created_at
		FROM version_approval_lists
	WHERE version_id = $1 AND de
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

func (db *PGCon) SaveApprovalListSettings(ctx c.Context, in e.SaveApprovalListSettings) (id string, err error) {
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
		in.VersionId,
		in.Name,
		in.Steps,
		in.ContextMapping,
		in.FormsMapping)
	if err != nil {
		return "", err
	}

	return listID, nil
}
