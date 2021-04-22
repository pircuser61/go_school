-- Database generated with pgModeler (PostgreSQL Database Modeler).
-- pgModeler  version: 0.9.2
-- PostgreSQL version: 12.0
-- Project Site: pgmodeler.io
-- Model Author: ---


-- Database creation must be done outside a multicommand file.
-- These commands were put in this file only as a convenience.
-- -- object: postgres | type: DATABASE --
-- -- DROP DATABASE IF EXISTS postgres;
-- CREATE DATABASE postgres
-- 	ENCODING = 'UTF8'
-- 	LC_COLLATE = 'en_US.utf8'
-- 	LC_CTYPE = 'en_US.utf8'
-- 	TABLESPACE = pg_default
-- 	OWNER = postgres;
-- -- ddl-end --
-- COMMENT ON DATABASE postgres IS E'default administrative connection database';
-- -- ddl-end --
--

-- object: pipeliner | type: SCHEMA --
-- DROP SCHEMA IF EXISTS pipeliner CASCADE;
CREATE SCHEMA pipeliner;
-- ddl-end --
-- ALTER SCHEMA pipeliner OWNER TO postgres;
-- ddl-end --

SET search_path TO pg_catalog,public,pipeliner;
-- ddl-end --

-- object: pipeliner.pipelines | type: TABLE --
-- DROP TABLE IF EXISTS pipeliner.pipelines CASCADE;
CREATE TABLE pipeliner.pipelines (
	id uuid NOT NULL,
	name character varying NOT NULL,
	created_at timestamp with time zone NOT NULL,
	deleted_at timestamp with time zone,
	author uuid NOT NULL,
	CONSTRAINT pipelines_pkey PRIMARY KEY (id)

);
-- ddl-end --
-- ALTER TABLE pipeliner.pipelines OWNER TO postgres;
-- ddl-end --

-- object: pipeliner.versions | type: TABLE --
-- DROP TABLE IF EXISTS pipeliner.versions CASCADE;
CREATE TABLE pipeliner.versions (
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
-- ddl-end --
-- ALTER TABLE pipeliner.versions OWNER TO postgres;
-- ddl-end --

-- object: pipeliner.version_status | type: TABLE --
-- DROP TABLE IF EXISTS pipeliner.version_status CASCADE;
CREATE TABLE pipeliner.version_status (
	id smallint NOT NULL,
	name character varying,
	CONSTRAINT version_status_pk PRIMARY KEY (id)

);
-- ddl-end --
-- ALTER TABLE pipeliner.version_status OWNER TO postgres;
-- ddl-end --

INSERT INTO pipeliner.version_status (id, name) VALUES (E'1', E'draft');
-- ddl-end --
INSERT INTO pipeliner.version_status (id, name) VALUES (E'2', E'approved');
-- ddl-end --
INSERT INTO pipeliner.version_status (id, name) VALUES (E'3', E'deleted');
-- ddl-end --
INSERT INTO pipeliner.version_status (id, name) VALUES (E'4', E'rejected');
-- ddl-end --

-- object: pipeliner.tags | type: TABLE --
-- DROP TABLE IF EXISTS pipeliner.tags CASCADE;
CREATE TABLE pipeliner.tags (
	id uuid NOT NULL,
	name character varying NOT NULL,
	status smallint NOT NULL,
	author uuid,
	id_tag_status smallint,
	CONSTRAINT tags_pk PRIMARY KEY (id)

);
-- ddl-end --
-- ALTER TABLE pipeliner.tags OWNER TO postgres;
-- ddl-end --

-- object: version_status_fk | type: CONSTRAINT --
-- ALTER TABLE pipeliner.versions DROP CONSTRAINT IF EXISTS version_status_fk CASCADE;
ALTER TABLE pipeliner.versions ADD CONSTRAINT version_status_fk FOREIGN KEY (id_version_status)
REFERENCES pipeliner.version_status (id) MATCH FULL
ON DELETE RESTRICT ON UPDATE CASCADE;
-- ddl-end --

-- object: pipelines_fk | type: CONSTRAINT --
-- ALTER TABLE pipeliner.versions DROP CONSTRAINT IF EXISTS pipelines_fk CASCADE;
ALTER TABLE pipeliner.versions ADD CONSTRAINT pipelines_fk FOREIGN KEY (id_pipelines)
REFERENCES pipeliner.pipelines (id) MATCH FULL
ON DELETE RESTRICT ON UPDATE CASCADE;
-- ddl-end --

-- object: pipeliner.many_tags_has_many_pipelines | type: TABLE --
-- DROP TABLE IF EXISTS pipeliner.many_tags_has_many_pipelines CASCADE;
CREATE TABLE pipeliner.many_tags_has_many_pipelines (
	id_tags uuid NOT NULL,
	id_pipelines uuid NOT NULL,
	CONSTRAINT many_tags_has_many_pipelines_pk PRIMARY KEY (id_tags,id_pipelines)

);
-- ddl-end --

-- object: tags_fk | type: CONSTRAINT --
-- ALTER TABLE pipeliner.many_tags_has_many_pipelines DROP CONSTRAINT IF EXISTS tags_fk CASCADE;
ALTER TABLE pipeliner.many_tags_has_many_pipelines ADD CONSTRAINT tags_fk FOREIGN KEY (id_tags)
REFERENCES pipeliner.tags (id) MATCH FULL
ON DELETE RESTRICT ON UPDATE CASCADE;
-- ddl-end --

-- object: pipelines_fk | type: CONSTRAINT --
-- ALTER TABLE pipeliner.many_tags_has_many_pipelines DROP CONSTRAINT IF EXISTS pipelines_fk CASCADE;
ALTER TABLE pipeliner.many_tags_has_many_pipelines ADD CONSTRAINT pipelines_fk FOREIGN KEY (id_pipelines)
REFERENCES pipeliner.pipelines (id) MATCH FULL
ON DELETE RESTRICT ON UPDATE CASCADE;
-- ddl-end --

-- object: pipeliner.tag_status | type: TABLE --
-- DROP TABLE IF EXISTS pipeliner.tag_status CASCADE;
CREATE TABLE pipeliner.tag_status (
	id smallint NOT NULL,
	name character varying NOT NULL,
	CONSTRAINT tag_status_pk PRIMARY KEY (id)

);
-- ddl-end --
-- ALTER TABLE pipeliner.tag_status OWNER TO postgres;
-- ddl-end --

INSERT INTO pipeliner.tag_status (id, name) VALUES (E'1', E'created');
-- ddl-end --
INSERT INTO pipeliner.tag_status (id, name) VALUES (E'2', E'approved');
-- ddl-end --
INSERT INTO pipeliner.tag_status (id, name) VALUES (E'3', E'deleted');
-- ddl-end --
INSERT INTO pipeliner.tag_status (id, name) VALUES (E'4', E'rejected');
-- ddl-end --

-- object: tag_status_fk | type: CONSTRAINT --
-- ALTER TABLE pipeliner.tags DROP CONSTRAINT IF EXISTS tag_status_fk CASCADE;
ALTER TABLE pipeliner.tags ADD CONSTRAINT tag_status_fk FOREIGN KEY (id_tag_status)
REFERENCES pipeliner.tag_status (id) MATCH FULL
ON DELETE SET NULL ON UPDATE CASCADE;
-- ddl-end --

-- object: pipeliner.pipeline_history | type: TABLE --
-- DROP TABLE IF EXISTS pipeliner.pipeline_history CASCADE;
CREATE TABLE pipeliner.pipeline_history (
	id uuid NOT NULL,
	pipeline_id uuid NOT NULL,
	version_id uuid NOT NULL,
	date timestamp with time zone NOT NULL,
	approver uuid NOT NULL,
	id_pipelines uuid,
	id_versions uuid,
	CONSTRAINT pipeline_history_pk PRIMARY KEY (id)

);
-- ddl-end --
-- ALTER TABLE pipeliner.pipeline_history OWNER TO postgres;
-- ddl-end --

-- object: pipelines_fk | type: CONSTRAINT --
-- ALTER TABLE pipeliner.pipeline_history DROP CONSTRAINT IF EXISTS pipelines_fk CASCADE;
ALTER TABLE pipeliner.pipeline_history ADD CONSTRAINT pipelines_fk FOREIGN KEY (id_pipelines)
REFERENCES pipeliner.pipelines (id) MATCH FULL
ON DELETE SET NULL ON UPDATE CASCADE;
-- ddl-end --

-- object: versions_fk | type: CONSTRAINT --
-- ALTER TABLE pipeliner.pipeline_history DROP CONSTRAINT IF EXISTS versions_fk CASCADE;
ALTER TABLE pipeliner.pipeline_history ADD CONSTRAINT versions_fk FOREIGN KEY (id_versions)
REFERENCES pipeliner.versions (id) MATCH FULL
ON DELETE SET NULL ON UPDATE CASCADE;
-- ddl-end --

-- object: pipeliner.works | type: TABLE --
-- DROP TABLE IF EXISTS pipeliner.works CASCADE;
CREATE TABLE pipeliner.works (
	id uuid NOT NULL,
	version_id uuid NOT NULL,
	started_at timestamp with time zone NOT NULL,
	status smallint NOT NULL,
	author uuid NOT NULL,
	id_versions uuid,
	id_work_status smallint,
	CONSTRAINT works_pk PRIMARY KEY (id)

);
-- ddl-end --
-- ALTER TABLE pipeliner.works OWNER TO postgres;
-- ddl-end --

-- object: versions_fk | type: CONSTRAINT --
-- ALTER TABLE pipeliner.works DROP CONSTRAINT IF EXISTS versions_fk CASCADE;
ALTER TABLE pipeliner.works ADD CONSTRAINT versions_fk FOREIGN KEY (id_versions)
REFERENCES pipeliner.versions (id) MATCH FULL
ON DELETE SET NULL ON UPDATE CASCADE;
-- ddl-end --

-- object: pipeliner.work_status | type: TABLE --
-- DROP TABLE IF EXISTS pipeliner.work_status CASCADE;
CREATE TABLE pipeliner.work_status (
	id smallint NOT NULL,
	name character varying NOT NULL,
	CONSTRAINT work_status_pk PRIMARY KEY (id)

);
-- ddl-end --
-- ALTER TABLE pipeliner.work_status OWNER TO postgres;
-- ddl-end --

INSERT INTO pipeliner.work_status (id, name) VALUES (E'1', E'started');
-- ddl-end --
INSERT INTO pipeliner.work_status (id, name) VALUES (E'2', E'finished');
-- ddl-end --
INSERT INTO pipeliner.work_status (id, name) VALUES (E'3', E'error');
-- ddl-end --

-- object: work_status_fk | type: CONSTRAINT --
-- ALTER TABLE pipeliner.works DROP CONSTRAINT IF EXISTS work_status_fk CASCADE;
ALTER TABLE pipeliner.works ADD CONSTRAINT work_status_fk FOREIGN KEY (id_work_status)
REFERENCES pipeliner.work_status (id) MATCH FULL
ON DELETE SET NULL ON UPDATE CASCADE;
-- ddl-end --

-- object: pipeliner.variable_storage | type: TABLE --
-- DROP TABLE IF EXISTS pipeliner.variable_storage CASCADE;
CREATE TABLE pipeliner.variable_storage (
	id uuid NOT NULL,
	work_id uuid NOT NULL,
	step_name character varying NOT NULL,
	"time" timestamp with time zone,
	content jsonb NOT NULL,
	id_works uuid,
	CONSTRAINT variable_storage_pk PRIMARY KEY (id)

);
-- ddl-end --
-- ALTER TABLE pipeliner.variable_storage OWNER TO postgres;
-- ddl-end --

-- object: works_fk | type: CONSTRAINT --
-- ALTER TABLE pipeliner.variable_storage DROP CONSTRAINT IF EXISTS works_fk CASCADE;
ALTER TABLE pipeliner.variable_storage ADD CONSTRAINT works_fk FOREIGN KEY (id_works)
REFERENCES pipeliner.works (id) MATCH FULL
ON DELETE SET NULL ON UPDATE CASCADE;
-- ddl-end --

-- object: pipeliner.log_storage | type: TABLE --
-- DROP TABLE IF EXISTS pipeliner.log_storage CASCADE;
CREATE TABLE pipeliner.log_storage (
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
-- ddl-end --
-- ALTER TABLE pipeliner.log_storage OWNER TO postgres;
-- ddl-end --

-- object: works_fk | type: CONSTRAINT --
-- ALTER TABLE pipeliner.log_storage DROP CONSTRAINT IF EXISTS works_fk CASCADE;
ALTER TABLE pipeliner.log_storage ADD CONSTRAINT works_fk FOREIGN KEY (id_works)
REFERENCES pipeliner.works (id) MATCH FULL
ON DELETE SET NULL ON UPDATE CASCADE;
-- ddl-end --

-- object: pipeliner.log_kind | type: TABLE --
-- DROP TABLE IF EXISTS pipeliner.log_kind CASCADE;
CREATE TABLE pipeliner.log_kind (
	id smallint NOT NULL,
	name character varying NOT NULL,
	CONSTRAINT log_kind_pk PRIMARY KEY (id)

);
-- ddl-end --
-- ALTER TABLE pipeliner.log_kind OWNER TO postgres;
-- ddl-end --

INSERT INTO pipeliner.log_kind (id, name) VALUES (E'1', E'error');
-- ddl-end --
INSERT INTO pipeliner.log_kind (id, name) VALUES (E'2', E'warning');
-- ddl-end --
INSERT INTO pipeliner.log_kind (id, name) VALUES (E'3', E'info');
-- ddl-end --

-- object: log_kind_fk | type: CONSTRAINT --
-- ALTER TABLE pipeliner.log_storage DROP CONSTRAINT IF EXISTS log_kind_fk CASCADE;
ALTER TABLE pipeliner.log_storage ADD CONSTRAINT log_kind_fk FOREIGN KEY (id_log_kind)
REFERENCES pipeliner.log_kind (id) MATCH FULL
ON DELETE SET NULL ON UPDATE CASCADE;
-- ddl-end --

CREATE SEQUENCE pipeliner.alarm_for_ngsa_id_seq
    INCREMENT 1
    START 1012971
    MINVALUE 1
    MAXVALUE 9223372036854775807
    CACHE 1;

ALTER SEQUENCE pipeliner.alarm_for_ngsa_id_seq
    OWNER TO erius;


CREATE TABLE pipeliner.alarm_for_ngsa
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

ALTER TABLE pipeliner.alarm_for_ngsa
    OWNER to erius;

GRANT ALL ON TABLE pipeliner.alarm_for_ngsa TO erius;



-- Index: mo_identifier

-- DROP INDEX pipeliner.mo_identifier;

CREATE INDEX mo_identifier
    ON pipeliner.alarm_for_ngsa USING btree
    ("moIdentifier" COLLATE pg_catalog."default")
    TABLESPACE pg_default;