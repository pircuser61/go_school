-- +goose Up
-- +goose StatementBegin
update pipeliner.public.versions
set content =
        jsonb_set(content, '{pipeline,blocks,start_0,output,properties,initiator}', '{
          "type": "object",
          "format": "SsoPerson",
          "global": "start_0.initiator",
          "properties": {
            "email": {
              "type": "string"
            },
            "phone": {
              "type": "string"
            },
            "mobile": {
              "type": "string"
            },
            "tabnum": {
              "type": "string"
            },
            "fullname": {
              "type": "string"
            },
            "position": {
              "type": "string"
            },
            "username": {
              "type": "string"
            },
            "fullOrgUnit": {
              "type": "string"
            }
          }
        }');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- +goose StatementEnd
