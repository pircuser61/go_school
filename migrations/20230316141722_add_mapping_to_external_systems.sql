-- +goose Up
-- +goose StatementBegin
ALTER TABLE IF EXISTS external_systems
    ADD COLUMN IF NOT EXISTS input_mapping jsonb;

ALTER TABLE IF EXISTS external_systems
    ADD COLUMN IF NOT EXISTS output_mapping jsonb;

COMMENT ON COLUMN external_systems.input_mapping
    IS 'Маппинг данных, которые необходимы для старта процесса с данными, которые внешняя система отдаёт';

COMMENT ON COLUMN external_systems.output_mapping
    IS 'Маппинг данных, которые процесс отдаёт при завершинии с данными, которые внешняя система принимает';

COMMENT ON COLUMN external_systems.version_id
    IS 'ID версии пайплайна';

COMMENT ON COLUMN external_systems.system_id
    IS 'ID подключаемой системы, получается из сервиса integrations';

COMMENT ON COLUMN external_systems.input_schema
    IS 'JSON-схема данных, которые внешняя система передаёт в процесс';

COMMENT ON COLUMN external_systems.output_schema
    IS 'JSON-схема данных, которые внешняя система хочет получить из процесса';

COMMENT ON COLUMN version_settings.version_id
    IS 'ID версии пайплайна';

COMMENT ON COLUMN version_settings.start_schema
    IS 'JSON-схема старта пайплайна';

COMMENT ON COLUMN version_settings.end_schema
    IS 'JSON-схема конца пайплайна';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE IF EXISTS external_systems DROP COLUMN IF EXISTS input_mapping;

ALTER TABLE IF EXISTS external_systems DROP COLUMN IF EXISTS output_mapping;

COMMENT ON COLUMN external_systems.version_id IS NULL;

COMMENT ON COLUMN external_systems.system_id IS NULL;

COMMENT ON COLUMN external_systems.input_schema IS NULL;

COMMENT ON COLUMN external_systems.output_schema IS NULL;

COMMENT ON COLUMN version_settings.version_id IS NULL;

COMMENT ON COLUMN version_settings.start_schema IS NULL;

COMMENT ON COLUMN version_settings.end_schema IS NULL;
-- +goose StatementEnd
