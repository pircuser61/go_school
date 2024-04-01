-- +goose Up
-- +goose StatementBegin
UPDATE dict_actions
SET id = 'common'
WHERE id = 'add_approvers';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE dict_actions
SET id = 'add_approvers'
WHERE id = 'common';
-- +goose StatementEnd
