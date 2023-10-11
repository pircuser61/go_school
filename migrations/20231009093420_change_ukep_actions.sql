-- +goose Up
-- +goose StatementBegin
UPDATE members
SET actions = '{sign_start_work:primary,sign_reject:secondary}'
WHERE params -> 'sign_sign' ->> 'signature_type' = 'ukep';

INSERT INTO dict_actions (id, title, is_public, comment_enabled, attachments_enabled)
VALUES ('sign_start_work', 'Подписать', true, false, false);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE members
SET actions = '{sign_sign:primary,sign_reject:secondary}'
WHERE params -> 'sign_sign' ->> 'signature_type' = 'ukep';

DELETE FROM dict_actions
WHERE id = 'sign_start_work';
-- +goose StatementEnd
