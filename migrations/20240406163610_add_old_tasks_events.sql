-- +goose Up
-- +goose StatementBegin
CREATE TABLE if not exists old_events AS
    (SELECT st.start_ids, st.started_at,pi.pause_ids,pi.fat, w.author
     FROM works w join
          (SELECT works.id as start_ids, works.started_at from works left join task_events te on works.id = te.work_id AND te.event_type = 'start' AND te.params = '{"steps": []}' where te.id is null)  st on w.id = st.start_ids join
          (SELECT works.id pause_ids, works.finished_at+interval '1 minute' as fat from works left join task_events te on works.id = te.work_id AND  te.event_type =  'pause' AND  te.params = '{"steps": []}'where te.id is null AND works.finished_at is not null) pi on pause_ids = start_ids);

INSERT INTO task_events (id, work_id, author, event_type, params, created_at)
    (SELECT uuid_generate_v4(), start_ids, author,'start', '{"steps": []}', started_at  from old_events);

INSERT INTO task_events (id, work_id, author, event_type,params, created_at)
    (SELECT uuid_generate_v4(), pause_ids, 'jocasta','pause', '{"steps": []}', fat  from old_events where pause_ids is not null);

drop table if exists old_events;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd
