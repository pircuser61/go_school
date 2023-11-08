-- +goose Up
-- +goose StatementBegin
drop aggregate if exists jsonb_merge_agg(jsonb);
drop function if exists jsonb_merge(jsonb, jsonb);

create or replace function jsonb_merge(a jsonb, b jsonb) returns jsonb
    language sql as $$
select
    jsonb_object_agg(
        coalesce(key_a, key_b),
        case
            when val_a isnull then val_b
            when val_b isnull then val_a
            when (jsonb_typeof(val_a) = 'array' and jsonb_typeof(val_b) = 'array') then
                array_to_json(array(select distinct el from jsonb_array_elements(val_a || val_b) as b(el)))::jsonb
            when (jsonb_typeof(val_a) <> 'object' or jsonb_typeof(val_b) <> 'object') then val_b
            else jsonb_merge(val_a, val_b)
        end
    )
from jsonb_each(a) e1(key_a, val_a)
full join jsonb_each(b) e2(key_b, val_b)
    on key_a = key_b
$$;

create or replace aggregate jsonb_merge_agg(jsonb)
(
    sfunc = jsonb_merge,
    stype = jsonb,
    initcond = '{}'
);

drop function if exists jsonb_array_to_text_array(jsonb);
-- +goose StatementEnd

-- +goose Down
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
