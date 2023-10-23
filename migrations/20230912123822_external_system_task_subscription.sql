-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS external_system_task_subscriptions
(
    id                  uuid PRIMARY KEY,
    version_id          uuid  NOT NULL,
    system_id           uuid  NOT NULL,
    microservice_id     uuid  NOT NULL,
    path                text  NOT NULL DEFAULT '',
    method              text  NOT NULL DEFAULT '',
    notification_schema jsonb NOT NULL DEFAULT '{}'::JSONB,
    mapping             jsonb NOT NULL DEFAULT '{}'::JSONB,
    nodes               jsonb NOT NULL DEFAULT '{}'::JSONB,
    CONSTRAINT external_system_task_subscriptions_unique_key UNIQUE (version_id, system_id),
    CONSTRAINT external_system_task_subscriptions_versions_fk FOREIGN KEY (version_id)
        REFERENCES versions (id) MATCH SIMPLE
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE external_system_task_subscriptions
-- +goose StatementEnd
