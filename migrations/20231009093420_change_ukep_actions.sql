-- +goose Up
-- +goose StatementBegin
UPDATE members
SET actions = '{sign_start_work:primary,sign_reject:secondary}'
WHERE params -> 'sign_sign' ->> 'signature_type' = 'ukep';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE members
SET actions = '{sign_sign:primary,sign_reject:secondary}'
WHERE params -> 'sign_sign' ->> 'signature_type' = 'ukep';
-- +goose StatementEnd
