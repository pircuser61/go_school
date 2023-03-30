-- +goose Up
-- +goose StatementBegin
COMMENT ON MATERIALIZED VIEW events IS 'Вывод всех событий по ноде построчно';
COMMENT ON COLUMN events.step_id IS 'Уникальный номер каждого шага';
COMMENT ON COLUMN events."user" IS 'Пользователь совершивший событие';
COMMENT ON COLUMN events.log_type IS 'Указывает тип лога';
COMMENT ON COLUMN events.event_body IS 'Указывает, что именно сделал пользователь';
COMMENT ON COLUMN events.created_at IS 'Дата, когда событие появилось.';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
COMMENT ON MATERIALIZED VIEW events IS NULL ;
COMMENT ON COLUMN events.step_id IS NULL ;
COMMENT ON COLUMN events."user" IS NULL ;
COMMENT ON COLUMN events.log_type IS NULL ;
COMMENT ON COLUMN events.event_body IS NULL ;
COMMENT ON COLUMN events.created_at IS NULL ;
-- +goose StatementEnd
