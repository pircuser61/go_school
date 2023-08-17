-- +goose Up
-- +goose StatementBegin
UPDATE pipeliner.public.works w
SET human_status = (select case
                               when ww.human_status = 'approve-sign' THEN 'signing'
                               when ww.human_status = 'approve-signed' THEN 'signed'
                               else ww.human_status end
                    FROM pipeliner.public.works ww
                    WHERE ww.id = w.id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE pipeliner.public.works w
SET human_status = (select case
                               when ww.human_status = 'signing' THEN 'approve-sign'
                               when ww.human_status = 'signed' THEN 'approve-signed'
                               else ww.human_status end
                    FROM pipeliner.public.works ww
                    WHERE ww.id = w.id);
-- +goose StatementEnd
