-- +goose Up
-- +goose StatementBegin
DELETE FROM deadlines WHERE block_id in (SELECT id FROM variable_storage WHERE step_type = 'form') AND action = 'sla_day_before'
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- +goose StatementEnd
