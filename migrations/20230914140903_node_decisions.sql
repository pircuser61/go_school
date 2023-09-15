-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS dict_node_decisions (
    id uuid NOT NULL PRIMARY KEY,
    node_type TEXT,
    decision TEXT,
    decision_title TEXT
);

INSERT INTO dict_node_decisions (id, node_type, decision, decision_title)
VALUES
    (uuid_generate_v4(), 'execution', 'executed', 'Исполнено'),
    (uuid_generate_v4(), 'execution', 'rejected', 'Отклонено'),
    (uuid_generate_v4(), 'execution', 'sent_edit', 'На доработку'),
    (uuid_generate_v4(), 'approver', 'approved', 'Согласовано'),
    (uuid_generate_v4(), 'approver', 'rejected', 'Отклонено'),
    (uuid_generate_v4(), 'approver', 'viewed', 'Ознакомлено'),
    (uuid_generate_v4(), 'approver', 'informed', 'Проинформировано'),
    (uuid_generate_v4(), 'approver', 'signed', 'Подписано'),
    (uuid_generate_v4(), 'approver', 'signed_ukep', 'Подписано УКЭП'),
    (uuid_generate_v4(), 'approver', 'confirmed', 'Утверждено'),
    (uuid_generate_v4(), 'approver', 'sent_to_edit', 'На доработку'),
    (uuid_generate_v4(), 'sign', 'signed', 'Подписано'),
    (uuid_generate_v4(), 'sign', 'rejected', 'Отклонено'),
    (uuid_generate_v4(), 'sign', 'error', 'Ошибка');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS dict_node_decisions;
-- +goose StatementEnd
