package handlers

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
	GetTasksError
	GetTaskError
	GetAllTagsError
	GetPipelineTagsError
	GetTagError
	TagCreateError
	TagEditError
	TagAttachError
	TagDeleteError
	TagParseError
	TagDetachError
	ModuleFindError
	PipelineIsNotDraft
	PipelineHasDraft
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
	GetVersionsByBlueprintIdError
	BodyParseError
	ValidationError
)

//nolint:dupl //its not duplicate
var errorText = map[Err]string{
	UnknownError:                  "unknown error",
	GetAllApprovedError:           "can't get approved versions",
	GetAllOnApproveError:          "can't get versions on approve",
	GetAllDraftsError:             "can't get draft versions",
	UUIDParsingError:              "can't find uuid",
	GetPipelineError:              "can't get pipeline",
	GetVersionError:               "can't get pipeline version",
	RequestReadError:              "can't read request",
	PipelineReadError:             "can't read pipeline data",
	PipelineWriteError:            "can't write pipeline data",
	PipelineParseError:            "can't pars pipeline data",
	PipelineIsDraft:               "pipeline is not a draft",
	ApproveError:                  "can't approve pipeline",
	PipelineDeleteError:           "can't delete pipeline data",
	PipelineCreateError:           "can't create pipeline",
	VersionCreateError:            "can't create version",
	ModuleUsageError:              "can't find function usage",
	PipelineRunError:              "can't run pipeline",
	Teapot:                        "nothing interest there",
	PipelineExecutionError:        "error when execution pipeline",
	PipelineOutputGrabError:       "error with output grabbing",
	UnauthError:                   "not allowed",
	AuthServiceError:              "auth service failed",
	GetTasksError:                 "can't find tasks",
	GetTaskError:                  "can't get task",
	GetAllTagsError:               "can't get all tags",
	GetPipelineTagsError:          "can't get pipeline tags",
	GetTagError:                   "can't get tag",
	TagCreateError:                "can't create tag",
	TagEditError:                  "can't edit tag",
	TagAttachError:                "can't attach tag",
	TagDeleteError:                "can't delete tag",
	TagParseError:                 "can't pars tag data",
	TagDetachError:                "can't detaсh tags from pipeline",
	ModuleFindError:               "can't find module",
	PipelineHasDraft:              "pipeline already has a draft",
	SchedulerClientFailed:         "scheduler client failed",
	NetworkMonitorClientFailed:    "network monitor client failed",
	ScenarioIsUsedInOtherError:    "scenario is used in other",
	PipelineNameUsed:              "pipeline name is already used",
	NoUserInContextError:          "no user in context",
	CreateDebugParseError:         "can't pars debug task data",
	CreateDebugInputsError:        "can't pars debug task inputs",
	CreateWorkError:               "can't create work",
	GetAllRejectedError:           "can't get rejected versions",
	RunDebugError:                 "error when execution debug pipeline",
	RunDebugTaskAlreadyRunError:   "can't start debug task with run status",
	RunDebugTaskFinishedError:     "can't start debug task with finished status",
	RunDebugTaskAlreadyError:      "can't start debug task with error status",
	RunDebugInvalidStatusError:    "can't start debug task with this status",
	GetUserinfoErr:                "can't get userinfo",
	BadFiltersError:               "can't parse filters",
	GetVersionsByBlueprintIdError: "can't get get versions by blueprintId",
	BodyParseError:                "can't parse body to struct",
	ValidationError:               "run version by blueprint id request is invalid",
}

// JOKE.
//nolint:dupl //its not duplicate
var errorDescription = map[Err]string{
	UnknownError:                  "Сохраняйте спокойствие, что-то произошло непонятное",
	GetAllApprovedError:           "Невозможно получить список согласованных сценариев",
	GetAllOnApproveError:          "Невозможно получить список сценариев, ожидающих согласования",
	GetAllDraftsError:             "Невозможно получить список редактируемых сценариев",
	UUIDParsingError:              "Не удалось прочитать идентификатор",
	GetPipelineError:              "Не удалось получить информацию о сценарии",
	GetVersionError:               "Не удалось получить информацию о сценарии",
	RequestReadError:              "Не удалось прочитать запрос",
	PipelineReadError:             "Не удалось прочитать информацию о сценарии",
	PipelineIsDraft:               "Редактирование согласованного сценария запрещено, необходимо создать новую версию",
	PipelineWriteError:            "Не удалось записать информацию о сценарии",
	PipelineParseError:            "Не удалось разобрать информацию о сценарии",
	ApproveError:                  "Не удалось согласовать сценарий",
	PipelineDeleteError:           "Не удалось удалить информацию о сценарии",
	PipelineCreateError:           "Не удалось создать информацию о сценарии",
	VersionCreateError:            "Не удалось создать версию сценария",
	ModuleUsageError:              "Ошибка при поиске использования функций в сценариях",
	PipelineRunError:              "Ошибка при запуске сценария",
	Teapot:                        "Мы заложили этот функционал, и сейчас он находится в реализации",
	PipelineExecutionError:        "При исполнении сценария произошла ошибка",
	PipelineOutputGrabError:       "Не удалось получить выходные данные",
	UnauthError:                   "Нет разрешений для выполнения операции",
	AuthServiceError:              "Ошибка сервиса авторизации",
	GetTasksError:                 "Не удалось найти запуски сценария",
	GetTaskError:                  "Не удалось получить экземпляр задачи",
	GetAllTagsError:               "Невозможно получить список тегов",
	GetPipelineTagsError:          "Невозможно получить список тегов сценария",
	GetTagError:                   "Не удалось получить информацию о теге",
	TagCreateError:                "Не удалось создать информацию о теге",
	TagEditError:                  "Не удалось изменить информацию о теге",
	TagAttachError:                "Не удалось прикрепить тег к сценарию",
	TagDeleteError:                "Не удалось удалить информацию о теге",
	TagParseError:                 "Не удалось разбрать информацию о теге",
	TagDetachError:                "Не удалось открепить тег от сценария",
	ModuleFindError:               "Не удалось найти функцию",
	PipelineHasDraft:              "Черновик данного сценария создан в разделе \"Мои сценарии\"",
	SchedulerClientFailed:         "Ошибка клиента планировщика",
	NetworkMonitorClientFailed:    "Ошибка клиента сетевого мониторинга",
	ScenarioIsUsedInOtherError:    "Невозможно удалить: сценарий используется в других сценариях",
	PipelineNameUsed:              "Сценарий с таким именем уже существует",
	NoUserInContextError:          "Пользователь не найден в контексте",
	CreateDebugParseError:         "Не удалось разобрать информацию о запуске сценария в режиме отладки",
	CreateDebugInputsError:        "Не удалось разобрать входные данные в режиме отладки",
	CreateWorkError:               "Не удалось создать новый запуск",
	GetAllRejectedError:           "Невозможно получить список сценариев, отправленных на доработку",
	RunDebugError:                 "При исполнении отладочного сценария произошла ошибка",
	RunDebugTaskAlreadyRunError:   "Невозможно запустить отладочный сценарий с статусом run",
	RunDebugTaskFinishedError:     "Невозможно запустить отладочный сценарий с статусом finished",
	RunDebugTaskAlreadyError:      "Невозможно запустить отладочный сценарий с статусом error",
	RunDebugInvalidStatusError:    "Невозможно запустить отладочный сценарий с таким статусом",
	GetUserinfoErr:                "Не удалось получить информацию о пользователе",
	BadFiltersError:               "Получены некорректные значения фильтров",
	GetVersionsByBlueprintIdError: "Ошибка при получении версий по id шаблона",
	BodyParseError:                "Ошибка при разборе тела запроса",
	ValidationError:               "Ошибка при валидации запроса",
}

var errorStatus = map[Err]int{
	Teapot:           http.StatusTeapot,
	UnauthError:      http.StatusUnauthorized,
	UUIDParsingError: http.StatusBadRequest,
	BadFiltersError:  http.StatusBadRequest,
	GetUserinfoErr:   http.StatusUnauthorized,
	BodyParseError:   http.StatusBadRequest,
	ValidationError:  http.StatusBadRequest,
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
