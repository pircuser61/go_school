-- +goose Up
-- +goose StatementBegin
alter table pipeliner.works rename column inputs to parameters;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
alter table pipeliner.works rename column parameters to inputs;
-- +goose StatementEnd
