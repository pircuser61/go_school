-- +goose Up
-- +goose StatementBegin
Update versions
set content = replace(content::text, '"attachments": {"type": "array", "items": {"type": "string"}, "description": "signed files"}',
    '"attachments": {"type": "array", "items": {"type": "object", "properties":{"file_id": {"type": "string", "description": "file id in file Registry"}, "external_link": {"type": "string", "description": "link to file in another system"}}}, "description": "signed files"}')::jsonb
where content::text ilike '%"attachments": {"type": "array", "items": {"type": "string"}, "description": "signed files"}%';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
Update versions
set content = replace(content::text, '"attachments": {"type": "array", "items": {"type": "object", "properties":{"file_id": {"type": "string", "description": "file id in file Registry"}, "external_link": {"type": "string", "description": "link to file in another system"}}}, "description": "signed files"}',
                      '"attachments": {"type": "array", "items": {"type": "string"}, "description": "signed files"}')::jsonb
where content::text ilike '%"attachments": {"type": "array", "items": {"type": "object", "properties":{"file_id": {"type": "string", "description": "file id in file Registry"}, "external_link": {"type": "string", "description": "link to file in another system"}}}, "description": "signed files"}%';
-- +goose StatementEnd