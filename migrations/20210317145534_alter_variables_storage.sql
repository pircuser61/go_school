-- +goose Up
-- +goose StatementBegin
alter table pipeliner.variable_storage drop constraint if exists works_fk;
-- +goose StatementEnd

-- +goose StatementBegin
alter table pipeliner.variable_storage drop column id_works;
-- +goose StatementEnd

-- +goose StatementBegin
alter table pipeliner.variable_storage
add constraint variable_storage_works_work_id_fk
foreign key (work_id) references pipeliner.works
on update set default on delete set null;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
alter table pipeliner.variable_storage drop constraint if exists variable_storage_works_work_id_fk;
-- +goose StatementEnd

-- +goose StatementBegin
alter table pipeliner.variable_storage
    add id_works uuid;
-- +goose StatementEnd

-- +goose StatementBegin
alter table pipeliner.variable_storage
add constraint variable_storage_works_id_works_fk
foreign key (id_works) references pipeliner.works
on update set default on delete set null;
-- +goose StatementEnd
