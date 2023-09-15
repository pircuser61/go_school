-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS decisions (
    id uuid NOT NULL PRIMARY KEY,
    node_type TEXT,
    decision TEXT,
    decision_title TEXT
);

INSERT INTO decisions (id, node_type, decision, decision_title)
VALUES
    (uuid_generate_v4(), 'Execution', 'executed', 'Исполнено'),
    (uuid_generate_v4(), 'Execution', 'rejected', 'Отклонено'),
    (uuid_generate_v4(), 'Execution', 'sent_edit', 'На доработку'),
    (uuid_generate_v4(), 'Approve', 'approved', 'Согласовано'),
    (uuid_generate_v4(), 'Approve', 'rejected', 'Отклонено'),
    (uuid_generate_v4(), 'Approve', 'viewed', 'Ознакомлено'),
    (uuid_generate_v4(), 'Approve', 'informed', 'Проинформировано'),
    (uuid_generate_v4(), 'Approve', 'signed', 'Подписано'),
    (uuid_generate_v4(), 'Approve', 'signed_ukep', 'Подписано УКЭП'),
    (uuid_generate_v4(), 'Approve', 'confirmed', 'Утверждено'),
    (uuid_generate_v4(), 'Approve', 'sent_to_edit', 'На доработку'),
    (uuid_generate_v4(), 'Sign', 'signed', 'Подписано'),
    (uuid_generate_v4(), 'Sign', 'rejected', 'Отклонено'),
    (uuid_generate_v4(), 'Sign', 'error', 'Ошибка');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS decisions;
-- +goose StatementEnd
