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
       case when step_type = 'approver' then ARRAY ['send_edit_app','add_approvers','request_add_info','approve','reject']
            when step_type = 'execution' then ARRAY ['executor_start_work','change_executor','request_execution_info']
            when step_type = 'form' then ARRAY ['fill_form']
            else ARRAY ['']
           end
from variable_storage
where members is not null;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
drop table if exists members
-- +goose StatementEnd
