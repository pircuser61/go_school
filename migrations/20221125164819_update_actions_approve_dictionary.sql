-- +goose Up
-- +goose StatementBegin
update dict_approve_action_names
set id = case
    when id = 'approved' then 'approve'
    when id = 'rejected' then 'reject'
    when id = 'affirmate' then 'confirm'
end
where id in ('approved', 'rejected', 'affirmate')
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
update dict_approve_action_names
set id = case
             when id = 'approve' then 'approved'
             when id = 'reject' then 'rejected'
             when id = 'confirm' then 'affirmate'
    end
where id in ('approve', 'reject', 'confirm')
-- +goose StatementEnd
