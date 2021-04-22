-- +goose Up
-- +goose StatementBegin
INSERT INTO pipeliner.work_status (id, name)
VALUES (E'1', E'run'),
       (E'4', E'stopped'),
       (E'5', E'created')
ON CONFLICT (id) DO UPDATE
    SET name=excluded.name;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM pipeliner.work_status WHERE id=E'4' OR id=E'5';
INSERT INTO pipeliner.work_status (id, name)
VALUES (E'1', E'started')
ON CONFLICT (id) DO UPDATE
    SET name=excluded.name;
-- +goose StatementEnd
