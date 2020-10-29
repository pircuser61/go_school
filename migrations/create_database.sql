
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