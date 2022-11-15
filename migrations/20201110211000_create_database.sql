-- +goose Up
CREATE TABLE pipelines (
                           id uuid NOT NULL,
                           name character varying NOT NULL,
                           created_at timestamp with time zone NOT NULL,
                           deleted_at timestamp with time zone,
                           author uuid NOT NULL,
                           CONSTRAINT pipelines_pkey PRIMARY KEY (id)

);

CREATE TABLE versions (
                          id uuid NOT NULL,
                          status smallint NOT NULL,
                          pipeline_id uuid NOT NULL,
                          created_at timestamp with time zone NOT NULL,
                          content jsonb NOT NULL,
                          author uuid NOT NULL,
                          approver uuid,
                          id_version_status smallint NOT NULL,
                          id_pipelines uuid NOT NULL,
                          CONSTRAINT versions_pk PRIMARY KEY (id)

);

CREATE TABLE version_status (
                                id smallint NOT NULL,
                                name character varying,
                                CONSTRAINT version_status_pk PRIMARY KEY (id)

);

INSERT INTO version_status (id, name) VALUES (E'1', E'draft');
INSERT INTO version_status (id, name) VALUES (E'2', E'approved');
INSERT INTO version_status (id, name) VALUES (E'3', E'deleted');
INSERT INTO version_status (id, name) VALUES (E'4', E'rejected');

CREATE TABLE tags (
                      id uuid NOT NULL,
                      name character varying NOT NULL,
                      status smallint NOT NULL,
                      author uuid,
                      id_tag_status smallint,
                      CONSTRAINT tags_pk PRIMARY KEY (id)

);

ALTER TABLE versions ADD CONSTRAINT version_status_fk FOREIGN KEY (id_version_status)
    REFERENCES version_status (id) MATCH FULL
    ON DELETE RESTRICT ON UPDATE CASCADE;

ALTER TABLE versions ADD CONSTRAINT pipelines_fk FOREIGN KEY (id_pipelines)
    REFERENCES pipelines (id) MATCH FULL
    ON DELETE RESTRICT ON UPDATE CASCADE;

CREATE TABLE many_tags_has_many_pipelines (
                                              id_tags uuid NOT NULL,
                                              id_pipelines uuid NOT NULL,
                                              CONSTRAINT many_tags_has_many_pipelines_pk PRIMARY KEY (id_tags,id_pipelines)

);

ALTER TABLE many_tags_has_many_pipelines ADD CONSTRAINT tags_fk FOREIGN KEY (id_tags)
    REFERENCES tags (id) MATCH FULL
    ON DELETE RESTRICT ON UPDATE CASCADE;

ALTER TABLE many_tags_has_many_pipelines ADD CONSTRAINT pipelines_fk FOREIGN KEY (id_pipelines)
    REFERENCES pipelines (id) MATCH FULL
    ON DELETE RESTRICT ON UPDATE CASCADE;

CREATE TABLE tag_status (
                            id smallint NOT NULL,
                            name character varying NOT NULL,
                            CONSTRAINT tag_status_pk PRIMARY KEY (id)

);

INSERT INTO tag_status (id, name) VALUES (E'1', E'created');
INSERT INTO tag_status (id, name) VALUES (E'2', E'approved');
INSERT INTO tag_status (id, name) VALUES (E'3', E'deleted');
INSERT INTO tag_status (id, name) VALUES (E'4', E'rejected');

ALTER TABLE tags ADD CONSTRAINT tag_status_fk FOREIGN KEY (id_tag_status)
    REFERENCES tag_status (id) MATCH FULL
    ON DELETE SET NULL ON UPDATE CASCADE;

CREATE TABLE pipeline_history (
                                  id uuid NOT NULL,
                                  pipeline_id uuid NOT NULL,
                                  version_id uuid NOT NULL,
                                  date timestamp with time zone NOT NULL,
                                  approver uuid NOT NULL,
                                  id_pipelines uuid,
                                  id_versions uuid,
                                  CONSTRAINT pipeline_history_pk PRIMARY KEY (id)

);

ALTER TABLE pipeline_history ADD CONSTRAINT pipelines_fk FOREIGN KEY (id_pipelines)
    REFERENCES pipelines (id) MATCH FULL
    ON DELETE SET NULL ON UPDATE CASCADE;

ALTER TABLE pipeline_history ADD CONSTRAINT versions_fk FOREIGN KEY (id_versions)
    REFERENCES versions (id) MATCH FULL
    ON DELETE SET NULL ON UPDATE CASCADE;

CREATE TABLE works (
                       id uuid NOT NULL,
                       version_id uuid NOT NULL,
                       started_at timestamp with time zone NOT NULL,
                       status smallint NOT NULL,
                       author uuid NOT NULL,
                       id_versions uuid,
                       id_work_status smallint,
                       CONSTRAINT works_pk PRIMARY KEY (id)

);

ALTER TABLE works ADD CONSTRAINT versions_fk FOREIGN KEY (id_versions)
    REFERENCES versions (id) MATCH FULL
    ON DELETE SET NULL ON UPDATE CASCADE;

CREATE TABLE work_status (
                             id smallint NOT NULL,
                             name character varying NOT NULL,
                             CONSTRAINT work_status_pk PRIMARY KEY (id)

);

INSERT INTO work_status (id, name) VALUES (E'1', E'started');
INSERT INTO work_status (id, name) VALUES (E'2', E'finished');
INSERT INTO work_status (id, name) VALUES (E'3', E'error');

ALTER TABLE works ADD CONSTRAINT work_status_fk FOREIGN KEY (id_work_status)
    REFERENCES work_status (id) MATCH FULL
    ON DELETE SET NULL ON UPDATE CASCADE;

CREATE TABLE variable_storage (
                                  id uuid NOT NULL,
                                  work_id uuid NOT NULL,
                                  step_name character varying NOT NULL,
                                  "time" timestamp with time zone,
                                  content jsonb NOT NULL,
                                  id_works uuid,
                                  CONSTRAINT variable_storage_pk PRIMARY KEY (id)

);

ALTER TABLE variable_storage ADD CONSTRAINT works_fk FOREIGN KEY (id_works)
    REFERENCES works (id) MATCH FULL
    ON DELETE SET NULL ON UPDATE CASCADE;

CREATE TABLE log_storage (
                             id uuid NOT NULL,
                             work_id uuid NOT NULL,
                             step_name character varying NOT NULL,
                             kind smallint NOT NULL,
                             "time" timestamp with time zone NOT NULL,
                             content character varying NOT NULL,
                             id_works uuid,
                             id_log_kind smallint,
                             CONSTRAINT log_storage_pk PRIMARY KEY (id)

);

ALTER TABLE log_storage ADD CONSTRAINT works_fk FOREIGN KEY (id_works)
    REFERENCES works (id) MATCH FULL
    ON DELETE SET NULL ON UPDATE CASCADE;

CREATE TABLE log_kind (
                          id smallint NOT NULL,
                          name character varying NOT NULL,
                          CONSTRAINT log_kind_pk PRIMARY KEY (id)

);

INSERT INTO log_kind (id, name) VALUES (E'1', E'error');
INSERT INTO log_kind (id, name) VALUES (E'2', E'warning');
INSERT INTO log_kind (id, name) VALUES (E'3', E'info');

ALTER TABLE log_storage ADD CONSTRAINT log_kind_fk FOREIGN KEY (id_log_kind)
    REFERENCES log_kind (id) MATCH FULL
    ON DELETE SET NULL ON UPDATE CASCADE;

CREATE SEQUENCE alarm_for_ngsa_id_seq
    INCREMENT 1
    START 1012971
    MINVALUE 1
    MAXVALUE 9223372036854775807
    CACHE 1;

ALTER SEQUENCE alarm_for_ngsa_id_seq
    OWNER TO erius;


CREATE TABLE alarm_for_ngsa
(
    state text COLLATE pg_catalog."default",
    "perceivedSeverity" integer,
    "eventSource" text COLLATE pg_catalog."default",
    "eventTime" timestamp without time zone,
    "eventType" text COLLATE pg_catalog."default",
    "probableCause" text COLLATE pg_catalog."default",
    "additionInformation" text COLLATE pg_catalog."default",
    "additionalText" text COLLATE pg_catalog."default",
    "moIdentifier" text COLLATE pg_catalog."default",
    "specificProblem" text COLLATE pg_catalog."default",
    "notificationIdentifier" text COLLATE pg_catalog."default",
    "userText" text COLLATE pg_catalog."default",
    managedobjectinstance text COLLATE pg_catalog."default",
    managedobjectclass text COLLATE pg_catalog."default",
    id integer NOT NULL DEFAULT nextval('alarm_for_ngsa_id_seq'::regclass),
    cleartime timestamp without time zone,
    CONSTRAINT primaty_id PRIMARY KEY (id),
    CONSTRAINT uniq_notification UNIQUE ("notificationIdentifier")

)
    WITH (
        OIDS = FALSE
    )
    TABLESPACE pg_default;

ALTER TABLE alarm_for_ngsa
    OWNER to erius;

GRANT ALL ON TABLE alarm_for_ngsa TO erius;



-- Index: mo_identifier

-- DROP INDEX mo_identifier;

CREATE INDEX mo_identifier
    ON alarm_for_ngsa USING btree
        ("moIdentifier" COLLATE pg_catalog."default")
    TABLESPACE pg_default;


-- +goose Down
DROP TABLE IF EXISTS pipelines CASCADE;

DROP TABLE IF EXISTS versions CASCADE;

DROP TABLE IF EXISTS versions_07092022 CASCADE;

DROP TABLE IF EXISTS version_status CASCADE;

DROP TABLE IF EXISTS tags CASCADE;

DROP TABLE IF EXISTS many_tags_has_many_pipelines CASCADE;

DROP TABLE IF EXISTS tag_status CASCADE;

DROP TABLE IF EXISTS pipeline_history CASCADE;

DROP TABLE IF EXISTS works CASCADE;

DROP TABLE IF EXISTS work_status CASCADE;

DROP TABLE IF EXISTS variable_storage CASCADE;

DROP TABLE IF EXISTS log_storage CASCADE;

DROP TABLE IF EXISTS log_kind CASCADE;

DROP SEQUENCE alarm_for_ngsa_id_seq CASCADE;

DROP TABLE IF EXISTS alarm_for_ngsa CASCADE;
