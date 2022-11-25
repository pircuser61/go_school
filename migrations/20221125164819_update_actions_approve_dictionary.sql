-- +goose Up
-- +goose StatementBegin
update dict_approve_action_names
set id = case
    when id = 'approved' then 'approve'
    when id = 'rejected' then 'reject'
end
where id in ('approved', 'rejected')
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
update dict_approve_action_names
set id = case
             when id = 'approve' then 'approved'
             when id = 'reject' then 'rejected'
    end
where id in ('approve', 'reject')
-- +goose StatementEnd
