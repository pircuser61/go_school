-- +goose Up
-- +goose StatementBegin
INSERT INTO work_status (id, name) VALUES (6, 'cancel') ON CONFLICT DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- +goose StatementEnd
