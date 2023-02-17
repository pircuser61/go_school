-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS version_settings
(
    id uuid NOT NULL,
    version_id uuid UNIQUE NOT NULL,
    start_schema jsonb NOT NULL DEFAULT '{}'::JSONB,
    end_schema jsonb NOT NULL DEFAULT '{}'::JSONB,
    CONSTRAINT version_settings_pkey PRIMARY KEY (id),
    CONSTRAINT version_settings_version_id_fkey FOREIGN KEY (version_id)
        REFERENCES versions (id) MATCH SIMPLE
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
);

GRANT ALL ON TABLE version_settings TO jocasta;

CREATE INDEX IF NOT EXISTS version_settings_version_idx
    ON version_settings USING btree
        (version_id ASC);

CREATE TABLE IF NOT EXISTS external_systems
(
    id uuid NOT NULL,
    version_id uuid NOT NULL,
    system_id uuid NOT NULL,
    input_schema jsonb NOT NULL DEFAULT '{}'::JSONB,
    output_schema jsonb NOT NULL DEFAULT '{}'::JSONB,
    CONSTRAINT external_systems_pkey PRIMARY KEY (id),
    CONSTRAINT external_systems_unique_key UNIQUE (version_id, system_id),
    CONSTRAINT external_systems_version_id_fkey FOREIGN KEY (version_id)
        REFERENCES versions (id) MATCH SIMPLE
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
);

GRANT ALL ON TABLE external_systems TO jocasta;

CREATE INDEX IF NOT EXISTS external_systems_version_idx
    ON external_systems USING btree
        (version_id ASC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS version_settings_version_idx;
DROP TABLE IF EXISTS version_settings;

DROP INDEX IF EXISTS external_systems_version_idx;
DROP TABLE IF EXISTS external_systems;
-- +goose StatementEnd
