package api

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type PipelinerError struct {
	Err
}

func (p *PipelinerError) Error() string {
	return p.error()
}

type Err int

const (
	UnknownError Err = iota
	UUIDParsingError
	RequestReadError
	PipelineParseError
	PipelineReadError
	PipelineWriteError
	GetAllApprovedError
	GetAllOnApproveError
	GetAllDraftsError
	GetPipelineError
	GetVersionError
	GetPipelineVersionsError
	PipelineIsDraft
	ApproveError
	PipelineDeleteError
	PipelineCreateError
	ModuleUsageError
	PipelineRunError
	Teapot
	PipelineExecutionError
	PipelineOutputGrabError
	VersionCreateError
	UnauthError
	AuthServiceError
	GetFormsChangelogError
	GetTasksError
	GetDelegationsError
	GetTaskError
	GetTasksCountError
	TokenParseError
	GetProcessSettingsError
	GetExternalSystemsError
	GetExternalSystemSettingsError
	GetExternalSystemsNamesError
	GetClientIDError
	ProcessSettingsSaveError
	ProcessSettingsParseError
	ProcessSettingsConvertError
	ExternalSystemSettingsSaveError
	ExternalSystemSettingsParseError
	ExternalSystemSettingsConvertError
	ExternalSystemAddingError
	ExternalSystemRemoveError
	JSONSchemaValidationError
	MappingError
	ModuleFindError
	PipelineIsNotDraft
	SchedulerClientFailed
	ScenarioIsUsedInOtherError
	PipelineNameUsed
	NoUserInContextError
	CreateDebugParseError
	CreateDebugInputsError
	CreateWorkError
	GetAllRejectedError
	RunDebugError
	RunDebugTaskAlreadyRunError
	RunDebugTaskFinishedError
	RunDebugTaskAlreadyError
	RunDebugInvalidStatusError
	NetworkMonitorClientFailed
	GetUserinfoErr
	BadFiltersError
	WorkNumberParsingError
	UpdateTaskParsingError
	UpdateTaskValidationError
	UpdateNotRunningTaskError
	UpdateBlockError
	BlockNotFoundError
	GetVersionsByBlueprintIdError
	BodyParseError
	ValidationError
	GetVersionsByWorkNumberError
	PipelineRenameParseError
	PipelineRenameError
	GetPipelinesSearchError
	ValidationPipelineSearchError
	UpdateTaskRateError
	ParseMailsError
	GetMonitoringNodesError
	GetBlockContextError
	GetTasksForMonitoringError
	GetTasksForMonitoringGetUserError
	NoProcessNodesForMonitoringError
	GetEntryPointOutputError
	UpdateEndingSystemSettingsError
	UpdateRunAsOthersSettingsError
	ValidationEndingSystemSettingsError
	SearchingForPipelinesUsageError
	ValidationSlaProcessSettingsError
	GetProcessSlaSettingsError
	PipelineValidateError
	StopTaskParsingError
	ParallelNodeReturnCycle
	ParallelNodeExitsNotConnected
	OutOfParallelNodesConnection
	ParallelOutOfStartInsert
	GetDecisionsError
	GetBlockStateError
	ParallelPathIntersected
	GetDeadlineError
)

//nolint:dupl //its not duplicate
var errorText = map[Err]string{
	UnknownError:                        "unknown error",
	GetAllApprovedError:                 "can't get approved versions",
	GetAllOnApproveError:                "can't get versions on approve",
	GetAllDraftsError:                   "can't get draft versions",
	UUIDParsingError:                    "can't find uuid",
	GetPipelineError:                    "can't get pipeline",
	GetVersionError:                     "can't get pipeline version",
	GetPipelineVersionsError:            "can't get pipeline versions",
	RequestReadError:                    "can't read request",
	PipelineReadError:                   "can't read pipeline data",
	PipelineWriteError:                  "can't write pipeline data",
	PipelineParseError:                  "can't pars pipeline data",
	PipelineIsDraft:                     "pipeline is not a draft",
	ApproveError:                        "can't approve pipeline",
	PipelineDeleteError:                 "can't delete pipeline data",
	PipelineCreateError:                 "can't create pipeline",
	VersionCreateError:                  "can't create version",
	ModuleUsageError:                    "can't find function usage",
	PipelineRunError:                    "can't run pipeline",
	Teapot:                              "nothing interest there",
	PipelineExecutionError:              "error when execution pipeline",
	PipelineOutputGrabError:             "error with output grabbing",
	UnauthError:                         "not allowed",
	AuthServiceError:                    "auth service failed",
	GetTasksError:                       "can't find tasks",
	GetTasksCountError:                  "can't get amount of tasks",
	GetTaskError:                        "can't get task",
	TokenParseError:                     "cant't parse token",
	GetProcessSettingsError:             "can't get process settings",
	GetExternalSystemsError:             "can't get external systems",
	GetExternalSystemSettingsError:      "can't get external system settings",
	GetExternalSystemsNamesError:        "can't get system names",
	GetClientIDError:                    "can't get ClientID",
	ProcessSettingsSaveError:            "can't save process settings",
	ProcessSettingsParseError:           "can't parse process settings data",
	ProcessSettingsConvertError:         "can't convert process settings data",
	ExternalSystemSettingsSaveError:     "can't save external system settings",
	ExternalSystemSettingsParseError:    "can't parse external systems settings data",
	ExternalSystemSettingsConvertError:  "can't convert external systems settings data",
	ExternalSystemAddingError:           "can't add external system to version",
	ExternalSystemRemoveError:           "can't remove external system from the list",
	JSONSchemaValidationError:           "json schema validation error",
	MappingError:                        "error occurred during data mapping",
	ModuleFindError:                     "can't find module",
	SchedulerClientFailed:               "scheduler client failed",
	NetworkMonitorClientFailed:          "network monitor client failed",
	ScenarioIsUsedInOtherError:          "scenario is used in other",
	PipelineNameUsed:                    "pipeline name is already used",
	NoUserInContextError:                "no user in context",
	CreateDebugParseError:               "can't pars debug task data",
	CreateDebugInputsError:              "can't pars debug task inputs",
	CreateWorkError:                     "can't create work",
	GetAllRejectedError:                 "can't get rejected versions",
	RunDebugError:                       "error when execution debug pipeline",
	RunDebugTaskAlreadyRunError:         "can't start debug task with run status",
	RunDebugTaskFinishedError:           "can't start debug task with finished status",
	RunDebugTaskAlreadyError:            "can't start debug task with error status",
	RunDebugInvalidStatusError:          "can't start debug task with this status",
	GetUserinfoErr:                      "can't get userinfo",
	BadFiltersError:                     "can't parse filters",
	WorkNumberParsingError:              "can't find work number",
	UpdateTaskParsingError:              "can't parse data to update task",
	UpdateTaskValidationError:           "can't validate data to update task",
	UpdateNotRunningTaskError:           "can't update not running work",
	UpdateBlockError:                    "can't update block",
	BlockNotFoundError:                  "can't find block",
	GetVersionsByBlueprintIdError:       "can't get get versions by blueprintId",
	BodyParseError:                      "can't parse body to struct",
	ValidationError:                     "run version by blueprint id request is invalid",
	GetVersionsByWorkNumberError:        "can`t find version by work id",
	PipelineRenameParseError:            "can't parse rename pipeline data",
	PipelineRenameError:                 "can't rename pipeline",
	GetPipelinesSearchError:             "can't find pipelines by search",
	ValidationPipelineSearchError:       "name and id are empty",
	GetFormsChangelogError:              "can't get forms history",
	UpdateTaskRateError:                 "can`t update task rate",
	ParseMailsError:                     "can`t parse mails",
	GetMonitoringNodesError:             "can't get nodes for monitoring",
	NoProcessNodesForMonitoringError:    "can't find nodes for monitoring",
	GetEntryPointOutputError:            "can't fill entry point output",
	UpdateEndingSystemSettingsError:     "can't update ending system settings",
	UpdateRunAsOthersSettingsError:      "failed to update settings for requests from a 3rd party",
	ValidationEndingSystemSettingsError: "not enough data to update ending settings",
	SearchingForPipelinesUsageError:     "can't find usages of pipeline",
	ValidationSlaProcessSettingsError:   "wrong data for version SLA settings",
	GetProcessSlaSettingsError:          "can't get sla settings for process",
	PipelineValidateError:               "invalid pipeline schema",
	StopTaskParsingError:                "can't parse stop task request",
	ParallelNodeReturnCycle:             "invalid pipeline schema: returning back from parallel",
	ParallelNodeExitsNotConnected:       "invalid pipeline schema: node exits are not connected",
	OutOfParallelNodesConnection:        "invalid pipeline schema: nodes outside of parallel connects with inside nodes",
	ParallelOutOfStartInsert:            "invalid pipeline schema: nodes outside of parallel connects with parallel end",
	GetDecisionsError:                   "can't get node decisions",
	GetBlockStateError:                  "can't get block state",
	ParallelPathIntersected:             "invalid pipeline schema: parallel path's are intersected",
	GetDeadlineError:                    "can't get deadline",
}

// JOKE.
//
//nolint:dupl //its not duplicate
var errorDescription = map[Err]string{
	UnknownError:                        "Сохраняйте спокойствие, что-то произошло непонятное",
	GetAllApprovedError:                 "Невозможно получить список согласованных сценариев",
	GetAllOnApproveError:                "Невозможно получить список сценариев, ожидающих согласования",
	GetAllDraftsError:                   "Невозможно получить список редактируемых сценариев",
	UUIDParsingError:                    "Не удалось прочитать идентификатор",
	GetPipelineError:                    "Не удалось получить информацию о сценарии",
	GetVersionError:                     "Не удалось получить информацию о сценарии",
	GetPipelineVersionsError:            "Не удалось получить информацию о версиях сценарии",
	RequestReadError:                    "Не удалось прочитать запрос",
	PipelineReadError:                   "Не удалось прочитать информацию о сценарии",
	PipelineIsDraft:                     "Редактирование согласованного сценария запрещено, необходимо создать новую версию",
	PipelineWriteError:                  "Не удалось записать информацию о сценарии",
	PipelineParseError:                  "Не удалось разобрать информацию о сценарии",
	ApproveError:                        "Не удалось согласовать сценарий",
	PipelineDeleteError:                 "Не удалось удалить информацию о сценарии",
	PipelineCreateError:                 "Не удалось создать информацию о сценарии",
	VersionCreateError:                  "Не удалось создать версию сценария",
	ModuleUsageError:                    "Ошибка при поиске использования функций в сценариях",
	PipelineRunError:                    "Ошибка при запуске сценария",
	Teapot:                              "Мы заложили этот функционал, и сейчас он находится в реализации",
	PipelineExecutionError:              "При исполнении сценария произошла ошибка",
	PipelineOutputGrabError:             "Не удалось получить выходные данные",
	UnauthError:                         "Нет разрешений для выполнения операции",
	AuthServiceError:                    "Ошибка сервиса авторизации",
	GetTasksError:                       "Не удалось найти запуски сценария",
	GetTasksCountError:                  "Не удалось получить количество задач",
	GetTaskError:                        "Не удалось получить экземпляр задачи",
	TokenParseError:                     "Не удалось разобрать токен",
	GetProcessSettingsError:             "Не удалось получить настройки процесса",
	GetExternalSystemsError:             "Не удалось получить подключенные внешние системы",
	GetExternalSystemSettingsError:      "Не удалось получить настройки внешней системы",
	GetExternalSystemsNamesError:        "Не удалось получить названия внешних систем",
	GetClientIDError:                    "Не удалось получить CliendID",
	ProcessSettingsSaveError:            "Не удалось сохранить настройки процесса",
	ProcessSettingsParseError:           "Не удалось получить данные из тела запроса",
	ProcessSettingsConvertError:         "Не удалось преобразовать данные из тела запроса",
	ExternalSystemSettingsSaveError:     "Не удалось сохранить настройки внешней системы",
	ExternalSystemSettingsParseError:    "Не удалось получить данные из тела запроса",
	ExternalSystemSettingsConvertError:  "Не удалось преобразовать данные из тела запроса",
	ExternalSystemAddingError:           "Не удалось подключить внешнюю систему к версии процесса",
	ExternalSystemRemoveError:           "Не удалось удалить внешнюю систему из списка подключенных",
	JSONSchemaValidationError:           "Ошибка валидации JSON-схемы",
	MappingError:                        "Произошла ошибка во время маппинга данных",
	ModuleFindError:                     "Не удалось найти функцию",
	SchedulerClientFailed:               "Ошибка клиента планировщика",
	NetworkMonitorClientFailed:          "Ошибка клиента сетевого мониторинга",
	ScenarioIsUsedInOtherError:          "Невозможно удалить: сценарий используется в других сценариях",
	PipelineNameUsed:                    "Сценарий с таким именем уже существует",
	NoUserInContextError:                "Пользователь не найден в контексте",
	CreateDebugParseError:               "Не удалось разобрать информацию о запуске сценария в режиме отладки",
	CreateDebugInputsError:              "Не удалось разобрать входные данные в режиме отладки",
	CreateWorkError:                     "Не удалось создать новый запуск",
	GetAllRejectedError:                 "Невозможно получить список сценариев, отправленных на доработку",
	RunDebugError:                       "При исполнении отладочного сценария произошла ошибка",
	RunDebugTaskAlreadyRunError:         "Невозможно запустить отладочный сценарий с статусом run",
	RunDebugTaskFinishedError:           "Невозможно запустить отладочный сценарий с статусом finished",
	RunDebugTaskAlreadyError:            "Невозможно запустить отладочный сценарий с статусом error",
	RunDebugInvalidStatusError:          "Невозможно запустить отладочный сценарий с таким статусом",
	GetUserinfoErr:                      "Не удалось получить информацию о пользователе",
	BadFiltersError:                     "Получены некорректные значения фильтров",
	WorkNumberParsingError:              "Не удалось прочитать идентификатор задачи",
	UpdateTaskParsingError:              "Не удалось прочитать информацию об обновлении задачи",
	UpdateTaskValidationError:           "Не удалось прочитать информацию об обновлении задачи",
	UpdateNotRunningTaskError:           "Невозможно обновить незапущенную задачу",
	UpdateBlockError:                    "Не удалось обновить блок задачи",
	BlockNotFoundError:                  "Не удалось получить блок задачи",
	GetVersionsByBlueprintIdError:       "Ошибка при получении версий по id шаблона",
	BodyParseError:                      "Ошибка при разборе тела запроса",
	ValidationError:                     "Ошибка при валидации запроса",
	GetVersionsByWorkNumberError:        "Ошибка при получении сценария по id процесса",
	PipelineRenameParseError:            "Не удалось получить информацию о переимоновании сценария",
	PipelineRenameError:                 "Не удалось переименовать сценарий",
	GetPipelinesSearchError:             "Не удалось найти сценарии в базе данных",
	ValidationPipelineSearchError:       "Не заполнены имя и айди сценария",
	GetFormsChangelogError:              "Не удалось получить историю изменения форм",
	UpdateTaskRateError:                 "Не удалось обновить оценку заявки",
	ParseMailsError:                     "Не удалось разобрать письма с действиями",
	GetMonitoringNodesError:             "Ошибка при получени нод для мониторинга в базе данных",
	NoProcessNodesForMonitoringError:    "Не удалось найти ноды для мониторинга даного процесса",
	GetEntryPointOutputError:            "Не удалось заполнить output стартовой ноды",
	UpdateEndingSystemSettingsError:     "Не удалось обновить настройки завершения процесса в системе",
	UpdateRunAsOthersSettingsError:      "Не удалось обновить настройки запуска заявки от третьего лица",
	ValidationEndingSystemSettingsError: "Ошибка при валидации параметров для обновления настроек системы",
	SearchingForPipelinesUsageError:     "Ошибка при поиске использования пайплайна",
	ValidationSlaProcessSettingsError:   "Ошибка при валидации параметров SLA процесса",
	GetProcessSlaSettingsError:          "Ошибка при получении параметров SLA процесса",
	PipelineValidateError:               "Невалидная схема пайплайна",
	StopTaskParsingError:                "Не удалось распарсить запрос",
	ParallelNodeReturnCycle:             "Линии блоков внутри параллельности должны быть изолированы",
	ParallelNodeExitsNotConnected:       "Процесс не опубликован. Соедините все ноды в процессе",
	// nolint
	OutOfParallelNodesConnection: "Процесс не опубликован. Есть ноды, которые не располагаются внутри параллельности или не проходят через начало/конец шлюза, но связаны с блоками внутри параллельности.",
	ParallelOutOfStartInsert:     "Процесс не опубликован. Есть ноды, которые соеденены с нодой конец параллельности, но не проходят через ноду начало параллельности",
	GetDecisionsError:            "Не удалось получить список решений нод",
	GetBlockStateError:           "can't get block state",
	ParallelPathIntersected:      "Процесс не опубликован. Внутри параллельности один из сокетов ведет на другую ветвь внутри параллельности",
}

var errorStatus = map[Err]int{
	Teapot:                        http.StatusTeapot,
	UnauthError:                   http.StatusUnauthorized,
	UUIDParsingError:              http.StatusBadRequest,
	BadFiltersError:               http.StatusBadRequest,
	GetUserinfoErr:                http.StatusUnauthorized,
	WorkNumberParsingError:        http.StatusBadRequest,
	UpdateTaskParsingError:        http.StatusBadRequest,
	UpdateTaskValidationError:     http.StatusBadRequest,
	UpdateNotRunningTaskError:     http.StatusBadRequest,
	BlockNotFoundError:            http.StatusBadRequest,
	BodyParseError:                http.StatusBadRequest,
	ValidationError:               http.StatusBadRequest,
	PipelineValidateError:         http.StatusBadRequest,
	StopTaskParsingError:          http.StatusBadRequest,
	ParallelNodeReturnCycle:       http.StatusBadRequest,
	ParallelNodeExitsNotConnected: http.StatusBadRequest,
	OutOfParallelNodesConnection:  http.StatusBadRequest,
}

type httpError struct {
	StatusCode  int    `json:"status_code"`
	Error       string `json:"error"`
	Description string `json:"description"`
}

func (c Err) errorMessage(e error) string {
	if e != nil {
		return fmt.Sprintf("%s: %s", c.error(), e.Error())
	}

	return c.error()
}

func (c Err) error() string {
	if s, ok := errorText[c]; ok {
		return s
	}

	return errorText[UnknownError]
}

func (c Err) status() int {
	if s, ok := errorStatus[c]; ok {
		return s
	}

	return http.StatusInternalServerError
}

func (c Err) description() string {
	if s, ok := errorDescription[c]; ok {
		return s
	}

	return errorDescription[UnknownError]
}

func (c Err) sendError(w http.ResponseWriter) error {
	resp := httpError{
		StatusCode:  c.status(),
		Error:       c.error(),
		Description: c.description(),
	}

	w.WriteHeader(resp.StatusCode)

	err := json.NewEncoder(w).Encode(resp)
	if err != nil {
		return err
	}

	return nil
}
