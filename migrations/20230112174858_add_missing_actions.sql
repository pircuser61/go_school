-- +goose Up
-- +goose StatementBegin
INSERT INTO dict_actions (id, title, is_public, comment_enabled, attachments_enabled)
VALUES
    ('decline', 'Отклонить', TRUE, TRUE, TRUE),
    ('executor_send_edit_app', 'Вернуть на доработку', TRUE, TRUE, TRUE);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM dict_actions
WHERE id IN ('decline', 'executor_send_edit_app')
-- +goose StatementEnd
