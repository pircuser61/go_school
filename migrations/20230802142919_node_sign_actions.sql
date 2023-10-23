-- +goose Up
-- +goose StatementBegin
insert into pipeliner.public.dict_actions(id, title, is_public, comment_enabled, attachments_enabled)
values ('sign_sign', 'Подписать', true, false, true),
       ('sign_reject', 'Отклонить', true, true, false);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
delete
from pipeliner.public.dict_actions
where id in ('sign_sign', 'sign_reject')
-- +goose StatementEnd
