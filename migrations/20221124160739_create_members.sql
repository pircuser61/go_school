-- +goose Up
-- +goose StatementBegin
create table if not exists members
(
    id uuid not null primary key ,
    block_id uuid not null,
    login varchar not null default '',
    finished boolean default false,
    actions varchar[] not null default '{}'
);

ALTER TABLE members ADD CONSTRAINT block_fk FOREIGN KEY (block_id)
    REFERENCES variable_storage (id) MATCH FULL
    ON DELETE RESTRICT ON UPDATE CASCADE;

CREATE INDEX IF NOT EXISTS index_logins on members (login);
CREATE INDEX IF NOT EXISTS index_finish on members (finished);

insert into members(id,block_id, login, finished, actions)
select uuid_generate_v4() ,id, unnest(members),
       case when status in ('finished', 'cancel') then true
            else false
           end,
       case when step_type = 'approver' AND status not in ('finished', 'cancel') then ARRAY ['send_edit_app:other','add_approvers:other','request_add_info:other','approve:primary','reject:secondary']
            when step_type = 'approver' AND status in ('finished', 'cancel') then ARRAY ['']
            when step_type = 'execution' AND status not in ('finished', 'cancel') AND content->'State'->step_name->>'execution_type' = 'group'  AND
                 content->'State'->step_name->>'is_taken_in_work' = 'false' then ARRAY ['executor_start_work:primary']
            when step_type = 'execution' AND status not in ('finished', 'cancel') AND content->'State'->step_name->>'execution_type' = 'group' AND
                 content->'State'->step_name->>'is_taken_in_work' = 'true' then ARRAY ['execution:primary', 'decline:secondary', 'change_executor:other','request_execution_info:other']
            when step_type = 'execution' AND status in ('finished', 'cancel') then ARRAY ['']
            when step_type = 'execution' AND status not in ('finished', 'cancel') AND content->'State'->step_name->>'execution_type' != 'group'
                then ARRAY ['execution:primary', 'decline:secondary', 'change_executor:other','request_execution_info:other']
            when step_type = 'form' AND status not in ('finished', 'cancel') then ARRAY ['fill_form:primary']
            when step_type = 'form' AND status in ('finished', 'cancel') then ARRAY ['']
            else ARRAY ['']
           end
from variable_storage
where members is not null;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
drop table if exists members
-- +goose StatementEnd
