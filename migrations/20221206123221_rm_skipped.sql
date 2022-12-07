-- +goose Up
-- +goose StatementBegin
DELETE FROM members WHERE block_id IN (SELECT id FROM variable_storage WHERE status = 'skipped');
ALTER TABLE members DROP CONSTRAINT block_fk;
DELETE FROM variable_storage WHERE status = 'skipped';
ALTER TABLE members ADD CONSTRAINT block_fk
    FOREIGN KEY (block_id) REFERENCES variable_storage (id)
        ON UPDATE CASCADE ON DELETE RESTRICT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- +goose StatementEnd
