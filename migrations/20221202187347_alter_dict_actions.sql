-- +goose Up
-- +goose StatementBegin

ALTER TABLE dict_actions
    ADD COLUMN priority integer;

UPDATE dict_actions SET priority = 1
    WHERE id = 'approve';

UPDATE dict_actions SET priority = 5
    WHERE id = 'reject';

UPDATE dict_actions SET priority = 10
    WHERE id = 'informed';

UPDATE dict_actions SET priority = 15
    WHERE id = 'confirm';

UPDATE dict_actions SET priority = 20
    WHERE id = 'sign';

UPDATE dict_actions SET priority = 25
    WHERE id = 'viewed';

UPDATE dict_actions SET priority = 30
    WHERE id = 'approver_send_edit_app';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE dict_actions
    DROP COLUMN IF EXISTS priority;
-- +goose StatementEnd
