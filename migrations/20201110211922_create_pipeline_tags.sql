-- +goose Up
-- SQL in this section is executed when the migration is applied.
CREATE TABLE IF NOT EXISTS pipeliner.pipeline_tags
(
    pipeline_id UUID NOT NULL
        CONSTRAINT pipelines_fk
            REFERENCES pipeliner.pipelines
            ON UPDATE CASCADE ON DELETE RESTRICT,
    tag_id      UUID NOT NULL
        CONSTRAINT tags_fk
            REFERENCES pipeliner.tags
            ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT comp_key_pipeline_id_tag_id PRIMARY KEY (pipeline_id, tag_id)
);

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.
DROP TABLE pipeliner.pipeline_tags;
