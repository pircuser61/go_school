-- +goose Up
-- +goose StatementBegin
create or replace function jsonb_array_to_text_array(p_input jsonb)
    returns text[]
    language sql
    immutable
as $$
select coalesce(array_agg(ary)::text[], '{}') from jsonb_array_elements_text(p_input) as ary;
$$;

create or replace function jsonb_merge(orig jsonb, delta jsonb)
    returns jsonb language sql as $$
select
    jsonb_object_agg(
            coalesce(keyOrig, keyDelta),
            case
                when valOrig isnull then valDelta
                when valDelta isnull then valOrig
                when (jsonb_typeof(valOrig) = 'array' and jsonb_typeof(valDelta) = 'array') then
                    array_to_json(array(select distinct v from unnest(jsonb_array_to_text_array(valOrig || valDelta)) as b(v)))::jsonb
                when (jsonb_typeof(valOrig) <> 'object' or jsonb_typeof(valDelta) <> 'object') then valDelta
                else jsonb_merge(valOrig, valDelta)
                end
        )
from jsonb_each(orig) e1(keyOrig, valOrig)
         full join jsonb_each(delta) e2(keyDelta, valDelta) on keyOrig = keyDelta
$$;

create or replace aggregate jsonb_merge_agg(jsonb)
(
    sfunc = jsonb_merge(jsonb, jsonb),
    stype = jsonb
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
drop aggregate jsonb_merge_agg(jsonb);
drop function jsonb_array_to_text_array;
drop function jsonb_merge;
-- +goose StatementEnd
