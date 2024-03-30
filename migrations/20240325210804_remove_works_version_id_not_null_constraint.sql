-- +goose Up
-- +goose StatementBegin
ALTER TABLE works ALTER version_id DROP not null; 
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE works ALTER version_id SET not null;
-- +goose StatementEnd
