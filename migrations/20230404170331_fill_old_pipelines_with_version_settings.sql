-- +goose Up
-- +goose StatementBegin
INSERT INTO version_settings(id, version_id, resubmission_period) SELECT uuid_generate_v4(), id, 0 FROM versions ON CONFLICT(version_id) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- +goose StatementEnd
