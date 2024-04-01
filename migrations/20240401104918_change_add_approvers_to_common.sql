-- +goose Up
-- +goose StatementBegin
UPDATE dict_actions
SET node_type = 'common'
WHERE id = 'add_approvers';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE dict_actions
SET node_type = 'approvement'
WHERE id = 'add_approvers';
-- +goose StatementEnd
