-- +goose Up
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS author_login_index ON works USING gist(author);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS author_login_index;
-- +goose StatementEnd
