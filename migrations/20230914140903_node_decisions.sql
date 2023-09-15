-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS decisions (
    id SERIAL NOT NULL PRIMARY KEY,
    node_type TEXT,
    decision TEXT,
    decision_rus TEXT
);

INSERT INTO decisions (node_type, decision, decision_rus)
VALUES
    ('Execution', 'executed', 'Исполнено'),
    ('Execution', 'rejected', 'Отклонено'),
    ('Execution', 'sent_edit', 'На доработку'),
    ('Approve', 'approved', 'Согласовано'),
    ('Approve', 'rejected', 'Отклонено'),
    ('Approve', 'viewed', 'Ознакомлено'),
    ('Approve', 'informed', 'Проинформировано'),
    ('Approve', 'signed', 'Подписано'),
    ('Approve', 'signed_ukep', 'Подписано УКЭП'),
    ('Approve', 'confirmed', 'Утверждено'),
    ('Approve', 'sent_to_edit', 'На доработку'),
    ('Sign', 'signed', 'Подписано'),
    ('Sign', 'rejected', 'Отклонено'),
    ('Sign', 'error', 'Ошибка');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS decisions;
-- +goose StatementEnd
