-- +goose Up
-- +goose StatementBegin
GRANT SELECT ON TABLE events, processes_new, steps TO bi;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
REVOKE SELECT ON TABLE events, processes_new, steps FROM bi;
-- +goose StatementEnd
