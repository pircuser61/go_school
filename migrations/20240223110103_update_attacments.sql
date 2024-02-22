-- +goose Up
-- SQL in this section is executed when the migration is applied.
UPDATE variable_storage
    SET attachments = (SELECT )

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.
UPDATE variable_storage SET attachments = 0;