-- +goose Up
-- +goose StatementBegin

update versions set content = replace(content::text, '"approved"','"approve"')::json where created_at < '2023-01-13 09:00:00.000 +0300';

update versions set content = replace(content::text, '"rejected"','"reject"')::json where created_at < '2023-01-13 09:00:00.000 +0300' ;

update versions set content = replace(content::text, '"Согласовано",','"Согласовать","actionType":"primary",')::json where created_at < '2023-01-13 09:00:00.000 +0300' ;

update versions set content = replace(content::text, '"Отклонено",','"Отклонить","actionType":"secondary",')::json where created_at < '2023-01-13 09:00:00.000 +0300';

update versions set content = replace(content::text, '"edit_app",','"edit_app","actionType":"other",')::json where created_at < '2023-01-13 09:00:00.000 +0300' ;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
update versions set content = replace(content::text, '"approve"','"approved"')::json where created_at < '2023-01-13 09:00:00.000 +0300';

update versions set content = replace(content::text, '"reject"','"rejected"')::json where created_at < '2023-01-13 09:00:00.000 +0300';

update versions set content = replace(content::text, '"Согласовать","actionType":"primary",','"Согласовано",')::json where created_at < '2023-01-13 09:00:00.000 +0300' ;

update versions set content = replace(content::text,'"Отклонить","actionType":"secondary",','"Отклонено",')::json where created_at < '2023-01-13 09:00:00.000 +0300';

update versions set content = replace(content::text, '"edit_app","actionType":"other",','"edit_app",')::json where created_at < '2023-01-13 09:00:00.000 +0300' ;

-- +goose StatementEnd
