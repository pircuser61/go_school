-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS PUBLIC.MATERIAL_TYPE (
    ID INTEGER PRIMARY KEY,
    NAME TEXT
);



CREATE TABLE IF NOT EXISTS PUBLIC.MATERIAL_STATUS (
    ID INTEGER PRIMARY KEY,
    NAME TEXT
);

CREATE TABLE IF NOT EXISTS PUBLIC.MATERIAL (
    UUID UUID PRIMARY KEY,
    TYPE INTEGER,
    STATUS INTEGER,
    TITLE TEXT,
    CONTENT TEXT,
    DT_CREATE TIMESTAMPTZ,
    DT_UPDATE TIMESTAMPTZ,
    FOREIGN KEY (TYPE) REFERENCES MATERIAL_TYPE (ID),
    FOREIGN KEY (STATUS) REFERENCES MATERIAL_STATUS (ID)
);

INSERT INTO MATERIAL_TYPE 
(ID, NAME) 
VALUES 
(1, 'статья'),
(2, 'видеоролик'),
(3, 'презентация') 
ON CONFLICT (ID) DO NOTHING;

INSERT INTO MATERIAL_STATUS 
(ID, NAME) 
VALUES (1, 'активный'),(2, 'архивный') 
ON CONFLICT (ID) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE TABLE IF EXISTS PUBLIC.MATERIAL;

DELETE TABLE IF EXISTS PUBLIC.MATERIAL_STATUS;

DELETE TABLE IF EXISTS PUBLIC.MATERIAL_TYPE;

-- +goose StatementEnd