-- +goose Up
-- +goose StatementBegin
CREATE TABLE pipeliner.dict_approve_action_names (
     id uuid NOT NULL,
     title character varying NOT NULL,
     status_processing_title character varying NOT NULL,
     status_decision_title character varying NOT NULL,
     created_at timestamp with time zone NOT NULL,
     deleted_at timestamp with time zone,
     CONSTRAINT dict_approve_action_pkey PRIMARY KEY (id)
);

INSERT INTO pipeliner.dict_approve_action_names (
         id,
         title,
         status_processing_title,
         status_decision_title,
         created_at
    )
    VALUES
           ('82f2324d-cea1-4024-99c1-674380483d39', 'Согласовать', 'На согласовании', 'Согласовано', now()),
           ('55fe7832-9109-45b0-883b-cfacc25d14ca', 'Отклонить', 'На согласовании', 'Отклонено', now()),
           ('a747532c-8a9d-42c7-98cc-07a341ca41c6', 'Утвердить', 'На утверждении', 'Утверждено', now()),
           ('cf75561b-965a-46d5-a806-b8d59d9bc69e', 'Ознакомлен', 'На ознакомлении', 'Ознакомлено', now()),
           ('96cdb5f7-d9af-453d-9292-f9d87339a059', 'Проинформирован', 'На информировании', 'Проинформировано', now()),
           ('43d16439-f7e3-4dbb-8431-3bd401f46d9b', 'Подписать', 'На подписании', 'Подписано', now());

CREATE TABLE pipeliner.dict_approve_statuses (
     id uuid NOT NULL,
     title character varying NOT NULL,
     created_at timestamp with time zone NOT NULL,
     deleted_at timestamp with time zone,
     CONSTRAINT dict_approve_statuses_pkey PRIMARY KEY (id)
);

INSERT INTO pipeliner.dict_approve_statuses (
        id,
        title,
        created_at
    )
    VALUES
    ('fc4a38de-387e-4695-aac6-83ee0d1bfb0c', 'На согласовании', now()),
    ('63ee0dc9-2c46-49aa-95dc-74ff84a1c49c', 'На утверждении', now()),
    ('7e9f19e9-e1d6-4515-9bee-8dd2e6989252', 'На ознакомлении', now()),
    ('63b0eceb-366f-4f5d-a811-cbfc567749e9', 'На информировании', now()),
    ('f733dd02-8506-4a65-a179-39e9ec26a64a', 'На подписании', now());
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS pipeliner.dict_approve_action_names;
DROP TABLE IF EXISTS pipeliner.dict_approve_statuses;
-- +goose StatementEnd
