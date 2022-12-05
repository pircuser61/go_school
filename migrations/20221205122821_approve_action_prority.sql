-- +goose Up
-- +goose StatementBegin

ALTER TABLE dict_approve_action_names
    ADD COLUMN priority integer;

UPDATE dict_approve_action_names SET priority = 1
WHERE id = 'approve';

UPDATE dict_approve_action_names SET priority = 5
WHERE id = 'reject';

UPDATE dict_approve_action_names SET priority = 10
WHERE id = 'informed';

UPDATE dict_approve_action_names SET priority = 15
WHERE id = 'confirm';

UPDATE dict_approve_action_names SET priority = 20
WHERE id = 'sign';

UPDATE dict_approve_action_names SET priority = 25
WHERE id = 'viewed';

UPDATE dict_approve_action_names SET priority = 30
WHERE id = 'approver_send_edit_app';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE dict_approve_action_names
    DROP COLUMN IF EXISTS priority;
-- +goose StatementEnd
