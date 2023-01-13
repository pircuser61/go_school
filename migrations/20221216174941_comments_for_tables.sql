-- +goose Up
-- +goose StatementBegin
comment on table pipelines is 'Таблица со всеми созданными сценариями.';

comment on column pipelines.id is 'Уникальный UUID идентификатор сценария.';
comment on constraint pipelines_pkey on pipelines is 'Первичный ключ сценария.';
comment on index pipelines_pkey is 'Индекс по первичному ключу сценария. Создан по умолчанию.';

comment on column pipelines.name is 'Название сценария.';
comment on constraint uniq_name on pipelines is 'Внешний ключ по названию сценария.';
comment on index uniq_name is 'Индекс созданный по внешнему ключу названия сценария.';

comment on column pipelines.author is 'Владелец сценария.';

comment on column pipelines.created_at is 'Поле для аудита. Дата создания записи в таблице.';
comment on column pipelines.deleted_at is 'Поле для аудита. Дата удаления записи в таблице.';

-- pipelines history table
comment on table pipeline_history is 'Таблица с историей изменений сценариев.';

comment on column pipeline_history.id is 'Уникальный идентификатор записи изменения сценария.';
comment on constraint pipeline_history_pk on pipeline_history is 'Первичный ключ по уникальному идентификатору записи изменения сценария.';
comment on index pipeline_history_pk is 'Индекс по первичному ключу - уникальному идентификатору записи изменения сценария.';

comment on column pipeline_history.pipeline_id is 'Уникальный идентификатор сценария.';
comment on column pipeline_history.version_id is 'Уникальный идентификатор версии сценария.';
comment on column pipeline_history.approver is 'НЕ ИСПОЛЬЗУЕТСЯ. Пришло из форка Erius.';
comment on column pipeline_history.id_pipelines is 'НЕ ИСПОЛЬЗУЕТСЯ. Пришло из форка Erius.';
comment on column pipeline_history.id_versions is 'НЕ ИСПОЛЬЗУЕТСЯ. Пришло из форка Erius.';

comment on column pipeline_history.date is 'Поле для аудита. Дата создания записи в таблице.';

-- versions table
comment on table versions is 'Таблица со всеми созданными версиями сценариев.';

comment on column versions.id is 'Уникальный идентификатор версии сценария.';
comment on constraint versions_pk on versions is 'Первичный ключ по уникальному идентификатору версии сценария.';
comment on index versions_pk is 'Индекс по первичному ключу - идентификатор версии сценария.';

comment on column versions.pipeline_id is 'Идентификатор сценария.';
comment on index versions_pipeline_id_index is 'Индекс по идентификатору сценария.';

comment on column versions.status is 'Статус версии сценария. 1 - черновик, 2 - опубликован, 3 - удален, 4 - отклонен (не используется), 5 - на согласовании (не используется)';
comment on column versions.content is 'JSON объект с метаданными блоков. На его основе производится запуск версии сценария в Pipeliner.';
comment on column versions.author is 'Владелец версии сценария.';
comment on column versions.approver is 'Человек, опубликовавший данную версию сценария.';
comment on column versions.comment_rejected is 'Не используется. Пришло из форка Erius.';
comment on column versions.comment is 'Не используется. Пришло из форка Erius.';
comment on column versions.last_run_id is 'Идентификатор последнего выполненной работы (works.id) по данной версии сценария.';
comment on column versions.is_actual is 'Флаг актуальности версии. В конкретную единицу времени может существовать только одна запись с флагом true.';

comment on column versions.created_at is 'Поле для аудита. Дата создания записи в таблице.';
comment on column versions.deleted_at is 'Поле для аудита. Дата удаления записи в таблице.';
comment on column versions.updated_at is 'Поле для аудита. Дата последнего обновления записи в таблице.';

-- version status table
comment on table version_status is 'Таблица-словарь со всеми статусами хранящихся в versions версий.';

comment on column version_status.id is 'Уникальный идентификатор статуса версии.';
comment on constraint version_status_pk on version_status is 'Первичный ключ уникального идентификатора статуса версии.';
comment on index version_status_pk is 'Индекс по первичному ключу - уникальный идентификатор статуса версии.';

comment on column version_status.name is 'Название статуса версии.';

-- works table
comment on table works is 'Таблица с данными заявок (a.k.a. "инстанс процесса") по версии сценария.';

comment on column works.id is 'Уникальный идентификатор запущенной заявки.';
comment on constraint works_pk on works is 'Первичный ключ по уникальному идентификатору запущенной заявки.';
comment on index works_pk is 'Индекс по первичному ключу - уникальному идентификатору запущенной заявки.';

comment on column works.work_number is 'Уникальный номер запущенной заявки по версии сценария.';
comment on index works_work_number_index is 'Индекс по номеру запущенной заявки.';
comment on index works_exp_index_filter is 'Индекс по номеру запущенной заявки.';

comment on column works.started_at is 'Дата запуска заявки по версии сценария.';
comment on index started_at_pr is 'Индекс по дате запуска заявки по версии сценария.';
comment on index works_started_at is 'Индекс по дате запуска заявки по версии сценария.';

comment on column works.version_id is 'Идентификатор версии сценария.';
comment on column works.status is 'Номерной статус заявки. Текст статуса можно получить в таблице work_status.';
comment on column works.author is 'Инициатор заявки.';
comment on column works.debug is 'Флаг того, что заявка запущена в тестовом режиме.';
comment on column works.parameters is 'НЕ ИСПОЛЬЗУЕТСЯ. Пришел из форка Erius.';
comment on column works.parent_work_id is 'НЕ ИСПОЛЬЗУЕТСЯ. Пришел из форка Erius.';
comment on column works.child_id is null;
comment on column works.active_blocks is 'Активные на текущий момент блоки.';
comment on column works.skipped_blocks is 'НЕ ИСПОЛЬЗУЕТСЯ. Ранее здесь указывались пропущенные блоки.';
comment on column works.notified_blocks is 'Блоки с уведомлением пользователей.';
comment on column works.prev_update_status_blocks is 'НЕ ИСПОЛЬЗУЕТСЯ. Ранее здесь указывались статусы блоков после обновления состояния извне.';
comment on column works.rate is 'Оценка выполненной заявки.';
comment on column works.rate_comment is 'Комментарий к оценке выполненной заявки.';

comment on column works.finished_at is 'Дата окончания работы экземпляра версии сценария.';

-- work status table
comment on table work_status is 'Таблица-словарь со статусами заявки.';

comment on column work_status.id is 'Уникальный идентификатор статуса заявки.';
comment on constraint work_status_pk on work_status is 'Первичный ключ по идентификатору статуса заявки.';
comment on index work_status_pk is 'Индекс по первичному ключу - идентификатору статуса заявки.';

comment on column work_status.name is 'Название статуса.';

-- variable storage table
comment on table variable_storage is 'Таблица, хранящая историю перехода между блоками заявки и их текущего бизнес-контекста.';

comment on column variable_storage.id is 'Уникальный идентификатор записи перехода блока.';
comment on constraint variable_storage_pk on variable_storage is 'Первичный ключ по идентификатору записи перехода блока.';
comment on index variable_storage_pk is 'Индекс по первичному ключу - идентификатору записи перехода блока.';

comment on column variable_storage.work_id is 'Уникальный идентификатор заявки.';
comment on index idx_variable_storage_work_id is 'Индекс по идентификатору заявки.';

comment on column variable_storage.status is 'Статус блока';
comment on index variable_storage_status_idx is 'Индекс по статусу блока.';

comment on column variable_storage.content is 'JSON описание всего текущего состояния бизнес процесса на момент нахождения в конкретном блоке.';
comment on index idxgin_content is 'Индекс по JSON содержимому.';

comment on column variable_storage.members is 'Участники процесса в конкретном блоке.';
comment on index index_members is 'Индекс по участникам процесса.';

comment on column variable_storage.step_type is 'Тип конкретного блока.';
comment on index variable_storage_step_type_idx is 'Индекс по типу блока.';

comment on column variable_storage.time is 'Время создания записи.';
comment on index variable_storage_time_index is 'Индекс по дате создания записи. Свежие данные идут в первую очередь.';

comment on index idx_variable_storage_work_id is 'Индекс по уникальному идентификатору заявки.';
comment on index variable_storage_work_id_step_type_status_index is 'Индекс по идентификатору заявки, статусу заявки и типу блока.';
comment on index count_index is 'Индекс по всем идентификаторам заявки с активным статусом.';

comment on column variable_storage.break_points is 'НЕ ИСПОЛЬЗУЕТСЯ. Пришло из форка Erius.';
comment on column variable_storage.step_name is 'Идентификатор текущего блока.';
comment on column variable_storage.has_error is 'Флаг того произошла ли ошибка в текущем блоке.';
comment on column variable_storage.check_sla is 'Флаг необходимости проверки SLA в текущем блоке.';
comment on column variable_storage.check_half_sla is 'Флаг необходимости проверки половины срока SLA в текущем блоке.';
comment on column variable_storage.sla_deadline is 'Дата окончания SLA по блоку.';

comment on column variable_storage.updated_at is 'Поле для аудита. Дата последнего обновления записи в таблице.';

-- dict_actions
comment on table dict_actions is 'Таблица-словарь со всеми доступными пользователю действиями в процессе.';

comment on column dict_actions.id is 'Уникальный идентификатор записи доступного действия.';
comment on constraint dict_actions_pkey on dict_actions is 'Первичный ключ по уникальному идентификатору записи доступного действия.';
comment on index dict_actions_pkey is 'Индекс по первичному ключу - уникальному идентификатору записи доступного действия.';

comment on column dict_actions.title is 'Название действия.';
comment on column dict_actions.attachments_enabled is 'Флаг того, можно ли во время этого действия прикладывать файлы.';
comment on column dict_actions.comment_enabled is 'Флаг того, можно ли во время этого действия оставлять комментарий.';
comment on column dict_actions.is_public is 'Флаг публичной доступности этого действия для внешних систем.';

-- dict_approve_action_names table
comment on table dict_approve_action_names is 'Таблица со всеми моделями представления пользовательских действий (для согласования). Используется в ServiceDesk.';

comment on column dict_approve_action_names.id is 'Уникальный идентификатор записи пользовательского действия.';
comment on constraint dict_approve_action_pkey on dict_approve_action_names is 'Первичный ключ по идентификатору пользовательского действия.';
comment on index dict_approve_action_pkey is 'Индекс по первичному ключу - идентификатору пользовательского действия.';

comment on column dict_approve_action_names.title is 'Название пользовательского действия.';
comment on column dict_approve_action_names.status_processing_title is 'Текст статуса во время ожидания исполнения пользовательского действия.';
comment on column dict_approve_action_names.status_decision_title is 'Текст статуса решения по пользовательскому действию.';
comment on column dict_approve_action_names.priority is 'Приоритет записи для упорядоченного вывода списка пользовательских действий.';

comment on column dict_approve_action_names.created_at is 'Поле для аудита. Дата создания записи в таблице.';
comment on column dict_approve_action_names.deleted_at is 'Поле для аудита. Дата удаления записи в таблице.';

-- dict_approve_statuses
comment on table dict_approve_statuses is 'Таблица со статусами пользовательских действий.';

comment on column dict_approve_statuses.id is 'Уникальный идентификатор статуса пользовательских действий.';
comment on constraint dict_approve_statuses_pkey on dict_approve_statuses is 'Первичный ключ по идентификатору статуса пользовательских действий.';
comment on index dict_approve_statuses_pkey is 'Индекс по первичному ключу - идентификатору статуса пользовательских действий.';

comment on column dict_approve_statuses.title is 'Текст статуса пользовательского действия.';

comment on column dict_approve_statuses.created_at is 'Поле для аудита. Дата создания записи в таблице.';
comment on column dict_approve_statuses.deleted_at is 'Поле для аудита. Дата удаления записи в таблице.';

-- members table
comment on table members is 'Таблица участников процесса в рамках конкретного блока.';

comment on column members.id is 'Уникальный идентификатор записи об участнике.';
comment on constraint members_pkey on members is 'Первичный ключ по уникальному идентификатору записи об участнике.';
comment on index members_pkey is 'Индекс по первичному ключу - уникальному идентификатору записи об участнике.';

comment on column members.login is 'Логин участника процесса в рамках блока.';
comment on index index_logins is 'Индекс по всем логинам участников процесса в рамках блока.';

comment on column members.finished is 'Флаг того закончено ли участие пользователя в блоке.';
comment on index index_finish is 'Индекс по всем законченным участникам.';

comment on column members.block_id is 'Уникальный идентификатор блока (variable_storage.id)';
comment on column members.actions is 'Массив доступных действий участнику блока.';

-- processes view
comment on view processes is 'Витрина с запущенными процессами';

comment on column processes.application_id is 'Идентификатор заявки.';
comment on column processes.process_name is 'Название сценария, по которому запущен процесс.';
comment on column processes.process_sla is 'НЕ ИСПОЛЬЗУЕТСЯ. SLA процесса.';
comment on column processes.block_sla is 'SLA текущего блока из процесса.';
comment on column processes.step_type is 'Тип текущего блока из процесса.';
comment on column processes.status is 'Статус текущего блока.';
comment on column processes.description is 'Описание текущего блока из процесса.';
comment on column processes.people is 'Участники текущего блока из процесса.';
comment on column processes.process_status is 'Статус процесса.';

comment on column processes.started_at is 'Время запуска заявки по процессу.';
comment on column processes.finished_at is 'Время окончания работы процесса по заявке.';
comment on column processes.process_finished_at is 'Время окончания работы процесса.';

-- log_storage table
comment on table log_storage is 'НЕ ИСПОЛЬЗУЕТСЯ. Пришло из форка Erius.';

-- log_kind table
comment on table log_kind is 'НЕ ИСПОЛЬЗУЕТСЯ. Пришло из форка Erius.';

-- pipeline tags table
comment on table pipeline_tags is 'НЕ ИСПОЛЬЗУЕТСЯ. Пришло из форка Erius.';

-- tags table
comment on table tags is 'НЕ ИСПОЛЬЗУЕТСЯ. Пришло из форка Erius.';

-- tags statuses table
comment on table tag_status is 'НЕ ИСПОЛЬЗУЕТСЯ. Пришло из форка Erius.';

-- versions_07092022 table
comment on table versions_07092022 is 'Не используется. Резервная копия версий до миграции на новые сокеты.';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- pipelines table
comment on table pipelines is null;

comment on column pipelines.id is null;
comment on constraint pipelines_pkey on pipelines is null;
comment on index pipelines_pkey is null;

comment on column pipelines.name is null;
comment on constraint uniq_name on pipelines is null;
comment on index uniq_name is null;

comment on column pipelines.author is null;

comment on column pipelines.created_at is null;
comment on column pipelines.deleted_at is null;

-- pipelines history table
comment on table pipeline_history is null;

comment on column pipeline_history.id is null;
comment on constraint pipeline_history_pk on pipeline_history is null;
comment on index pipeline_history_pk is null;

comment on column pipeline_history.pipeline_id is null;
comment on column pipeline_history.version_id is null;
comment on column pipeline_history.approver is null;
comment on column pipeline_history.id_pipelines is null;
comment on column pipeline_history.id_versions is null;

comment on column pipeline_history.date is null;

-- versions table
comment on table versions is null;

comment on column versions.id is null;
comment on constraint versions_pk on versions is null;
comment on index versions_pk is null;

comment on column versions.pipeline_id is null;
comment on index versions_pipeline_id_index is null;

comment on column versions.status is null;
comment on column versions.content is null;
comment on column versions.author is null;
comment on column versions.approver is null;
comment on column versions.comment_rejected is null;
comment on column versions.comment is null;
comment on column versions.last_run_id is null;
comment on column versions.is_actual is null;

comment on column versions.created_at is null;
comment on column versions.deleted_at is null;
comment on column versions.updated_at is null;

-- version status table
comment on table version_status is null;

comment on column version_status.id is null;
comment on constraint version_status_pk on version_status is null;
comment on index version_status_pk is null;

comment on column version_status.name is null;

-- works table
comment on table works is null;

comment on column works.id is null;
comment on constraint works_pk on works is null;
comment on index works_pk is null;

comment on column works.work_number is null;
comment on index works_work_number_index is null;
comment on index works_exp_index_filter is null;

comment on column works.started_at is null;
comment on index started_at_pr is null;
comment on index works_started_at is null;

comment on column works.version_id is null;
comment on column works.status is null;
comment on column works.author is null;
comment on column works.debug is null;
comment on column works.parameters is null;
comment on column works.parent_work_id is null;
comment on column works.child_id is null;
comment on column works.active_blocks is null;
comment on column works.skipped_blocks is null; -- не используется
comment on column works.notified_blocks is null;
comment on column works.prev_update_status_blocks is null;
comment on column works.rate is null;
comment on column works.rate_comment is null;

comment on column works.finished_at is null;

-- work status table
comment on table work_status is null;

comment on column work_status.id is null;
comment on constraint work_status_pk on work_status is null;
comment on index work_status_pk is null;

comment on column work_status.name is null;

-- variable storage table
comment on table variable_storage is null;

comment on column variable_storage.id is null;
comment on constraint variable_storage_pk on variable_storage is null;
comment on index variable_storage_pk is null;

comment on column variable_storage.work_id is null;
comment on index idx_variable_storage_work_id is null;

comment on column variable_storage.status is null;
comment on index variable_storage_status_idx is null;

comment on column variable_storage.content is null;
comment on index idxgin_content is null;

comment on column variable_storage.members is null;
comment on index index_members is null;

comment on column variable_storage.step_type is null;
comment on index variable_storage_step_type_idx is null;

comment on column variable_storage.time is null;
comment on index variable_storage_time_index is null;

comment on index idx_variable_storage_work_id is null;
comment on index variable_storage_work_id_step_type_status_index is null;
comment on index count_index is null;

comment on column variable_storage.break_points is null;
comment on column variable_storage.step_name is null;
comment on column variable_storage.has_error is null;
comment on column variable_storage.check_sla is null;
comment on column variable_storage.check_half_sla is null;
comment on column variable_storage.sla_deadline is null;

comment on column variable_storage.updated_at is null;

-- dict_actions
comment on table dict_actions is null;

comment on column dict_actions.id is null;
comment on constraint dict_actions_pkey on dict_actions is null;
comment on index dict_actions_pkey is null;

comment on column dict_actions.title is null;
comment on column dict_actions.attachments_enabled is null;
comment on column dict_actions.comment_enabled is null;
comment on column dict_actions.is_public is null;

-- dict_approve_action_names table
comment on table dict_approve_action_names is null;

comment on column dict_approve_action_names.id is null;
comment on constraint dict_approve_action_pkey on dict_approve_action_names is null;
comment on index dict_approve_action_pkey is null;

comment on column dict_approve_action_names.title is null;
comment on column dict_approve_action_names.status_processing_title is null;
comment on column dict_approve_action_names.status_decision_title is null;
comment on column dict_approve_action_names.priority is null;

comment on column dict_approve_action_names.created_at is null;
comment on column dict_approve_action_names.deleted_at is null;

-- dict_approve_statuses
comment on table dict_approve_statuses is null;

comment on column dict_approve_statuses.id is null;
comment on constraint dict_approve_statuses_pkey on dict_approve_statuses is null;
comment on index dict_approve_statuses_pkey is null;

comment on column dict_approve_statuses.title is null;

comment on column dict_approve_statuses.created_at is null;
comment on column dict_approve_statuses.deleted_at is null;

-- log_storage table
comment on table log_storage is null;

comment on column log_storage.id is null;
comment on constraint log_storage_pk on log_storage is null;
comment on index log_storage_pk is null;

comment on column log_storage.id_works is null;
comment on constraint works_fk on log_storage is null;

comment on column log_storage.id_log_kind is null;
comment on constraint log_kind_fk on log_storage is null;

comment on column log_storage.work_id is null;
comment on column log_storage.step_name is null;
comment on column log_storage.kind is null;
comment on column log_storage.content is null;

comment on column log_storage.time is null;

-- log_kind table
comment on table log_kind is null;

comment on column log_kind.id is null;
comment on constraint log_kind_pk on log_kind is null;
comment on index log_kind_pk is null;

comment on column log_kind.name is null;

-- members table
comment on table members is null;

comment on column members.id is null;
comment on constraint members_pkey on members is null;
comment on index members_pkey is null;

comment on column members.login is null;
comment on index index_logins is null;

comment on column members.finished is null;
comment on index index_finish is null;

comment on column members.block_id is null;
comment on column members.actions is null;

-- processes view
comment on view processes is null;

comment on column processes.application_id is null;
comment on column processes.process_name is null;
comment on column processes.process_sla is null;
comment on column processes.block_sla is null;
comment on column processes.step_type is null;
comment on column processes.status is null;
comment on column processes.description is null;
comment on column processes.people is null;
comment on column processes.process_status is null;

comment on column processes.started_at is null;
comment on column processes.finished_at is null;
comment on column processes.process_finished_at is null;

-- pipeline tags table
comment on table pipeline_tags is null;

-- tags table
comment on table tags is null;

-- tags statuses table
comment on table tag_status is null;

-- versions_07092022 table
comment on table versions_07092022 is null;
-- +goose StatementEnd
