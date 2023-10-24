-- +goose Up
-- +goose StatementBegin
alter table if exists public.dict_actions
    add column if not exists node_type varchar(16) not null default '';

do $$
declare action_id varchar(32) = '';
    begin
    for action_id in (select da.id from public.dict_actions da)
    loop
        update public.dict_actions da
            set node_type = case
                when action_id = 'execution' then 'execution'
                when action_id = 'executor_start_work' then 'execution'
                when action_id = 'change_executor' then 'execution'
                when action_id = 'request_execution_info' then 'execution'
                when action_id = 'executor_send_edit_app' then 'execution'
                when action_id = 'decline' then 'execution'

                when action_id = 'sign_ukep' then 'sign'
                when action_id = 'sign_reject' then 'sign'
                when action_id = 'sign_sign' then 'sign'
                when action_id = 'sign_start_work' then 'sign'

                when action_id = 'request_add_info' then 'approvement'
                when action_id = 'approve' then 'approvement'
                when action_id = 'reject' then 'approvement'
                when action_id = 'informed' then 'approvement'
                when action_id = 'confirm' then 'approvement'
                when action_id = 'sign' then 'approvement'
                when action_id = 'viewed' then 'approvement'
                when action_id = 'approver_send_edit_app' then 'approvement'
                when action_id = 'add_approvers' then 'approvement'
                when action_id = 'additional_reject' then 'approvement'
                when action_id = 'additional_approvement' then 'approvement'

                when action_id = 'fill_form' then 'form'
                when action_id = 'form_executor_start_work' then 'form'

                when action_id = 'cancel_app' then 'common'
                end
        where da.id = action_id;
    end loop;
end $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
alter table if exists public.dict_actions
    drop column if exists node_type;
-- +goose StatementEnd