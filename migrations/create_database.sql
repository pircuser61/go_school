
CREATE DATABASE postgres
WITH
    OWNER = postgres
    ENCODING = 'UTF8'
    LC_COLLATE = 'en_US.utf8'
    LC_CTYPE = 'en_US.utf8'
    TABLESPACE = pg_default
    CONNECTION LIMIT = -1;

COMMENT ON DATABASE postgres
    IS 'default administrative connection database';

ALTER DATABASE postgres
    SET search_path TO "$user", public, tiger;


CREATE SCHEMA public
    AUTHORIZATION postgres;

COMMENT ON SCHEMA public
    IS 'standard public schema';

GRANT ALL ON SCHEMA public TO postgres;

GRANT ALL ON SCHEMA public TO PUBLIC;



DROP TABLE IF EXISTS pipeliner.pipelines;
DROP TABLE IF EXISTS pipeliner.storage;
DROP TABLE IF EXISTS pipeliner.versions;

CREATE TABLE public.pipelines
(
    id uuid NOT NULL UNIQUE,
    name character varying COLLATE pg_catalog."default" NOT NULL UNIQUE,
    pipe jsonb NOT NULL,
    CONSTRAINT pipelines_pkey PRIMARY KEY (id)
)
    WITH (
        OIDS = FALSE
    )
    TABLESPACE pg_default;

ALTER TABLE public.pipelines
    OWNER to postgres;


CREATE TABLE public.storage
(
    id uuid NOT NULL,
    work_id uuid,
    pipeline_id uuid,
    stage character varying COLLATE pg_catalog."default",
    vars jsonb,
    CONSTRAINT storage_pkey PRIMARY KEY (id)
)
    WITH (
        OIDS = FALSE
    )
    TABLESPACE pg_default;

ALTER TABLE public.storage
    OWNER to postgres;


CREATE TABLE pipeliner.versions
(
    id uuid NOT NULL,
    status smallint NOT NULL,
    pipeline_id uuid NOT NULL,
    created_at timestamp with time zone NOT NULL,
    content jsonb NOT NULL,
    author character varying COLLATE pg_catalog."default",
    approver character varying COLLATE pg_catalog."default",
    deleted_at timestamp with time zone,
    CONSTRAINT versions_pk PRIMARY KEY (id)
)
    WITH (
        OIDS = FALSE
    )
    TABLESPACE pg_default;

ALTER TABLE pipeliner.versions
    OWNER to postgres;