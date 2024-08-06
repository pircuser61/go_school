-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS works_relations(
  work_number VARCHAR(20) NOT NULL PRIMARY KEY,
  parent_work_number TEXT,
  child_work_numbers TEXT[] NOT NULL DEFAULT '{}'
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS works_relations;
-- +goose StatementEnd
