package api

import (
	"net/http"

	conditions_kit "gitlab.services.mts.ru/jocasta/conditions-kit"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

const (
	WaitForAllInputsBase = "wait_for_all_inputs"
	IfBase               = "if"
	ConnectorBase        = "connector"
	ForBase              = "for"
	StringsIsEqualBase   = "strings_is_equal"
	BeginParallelTask    = "begin_parallel_task"
)

func (ae *Env) GetModules(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "list_modules")
	defer s.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	blocks := eriusFunctions()

	blocksResult := make([]script.FunctionModel, 0, len(blocks))

	for i := range blocks {
		blocks[i].Title = ae.eriusFunctionTitle(blocks[i].ID, blocks[i].Title)
		blocksResult = append(blocksResult, blocks[i])
	}

	eriusShapes, err := script.GetShapes()
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}

	err = sendResponse(w, http.StatusOK, entity.EriusFunctionList{Functions: blocksResult, Shapes: eriusShapes})
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

func eriusFunctions() []script.FunctionModel {
	return []script.FunctionModel{
		(&pipeline.GoSdApplicationBlock{}).Model(),
		(&pipeline.GoFormBlock{
			State: &pipeline.FormData{
				Executors:          make(map[string]struct{}, 0),
				InitialExecutors:   make(map[string]struct{}, 0),
				ApplicationBody:    make(map[string]interface{}, 0),
				Constants:          make(map[string]interface{}, 0),
				ChangesLog:         make([]pipeline.ChangesLogItem, 0),
				HiddenFields:       make([]string, 0),
				FormsAccessibility: make([]script.FormAccessibility, 0),
				Mapping:            make(map[string]script.JSONSchemaPropertiesValue, 0),
				AttachmentFields:   make([]string, 0),
				Keys:               make(map[string]string, 0),
			},
		}).Model(),
		(&pipeline.GoApproverBlock{
			State: &pipeline.ApproverData{
				ApproverLog:         make([]pipeline.ApproverLogEntry, 0),
				EditingAppLog:       make([]pipeline.ApproverEditingApp, 0),
				FormsAccessibility:  make([]script.FormAccessibility, 0),
				AddInfo:             make([]pipeline.AdditionalInfo, 0),
				ActionList:          make([]pipeline.Action, 0),
				AdditionalApprovers: make([]pipeline.AdditionalApprover, 0),
			},
		}).Model(),
		(&pipeline.GoExecutionBlock{
			State: &pipeline.ExecutionData{
				Executors:                make(map[string]struct{}, 0),
				InitialExecutors:         make(map[string]struct{}, 0),
				DecisionAttachments:      make([]entity.Attachment, 0),
				EditingAppLog:            make([]pipeline.ExecutorEditApp, 0),
				ChangedExecutorsLogs:     make([]pipeline.ChangeExecutorLog, 0),
				RequestExecutionInfoLogs: make([]pipeline.RequestExecutionInfoLog, 0),
				FormsAccessibility:       make([]script.FormAccessibility, 0),
				TakenInWorkLog:           make([]pipeline.StartWorkLog, 0),
			},
		}).Model(),
		(&pipeline.GoSignBlock{
			State: &pipeline.SignData{
				Signers:             make(map[string]struct{}, 0),
				Attachments:         make([]entity.Attachment, 0),
				Signatures:          make([]pipeline.FileSignaturePair, 0),
				SignLog:             make([]pipeline.SignLogEntry, 0),
				FormsAccessibility:  make([]script.FormAccessibility, 0),
				AdditionalApprovers: make([]pipeline.AdditionalSignApprover, 0),
			},
		}).Model(),
		(&pipeline.IF{
			State: &pipeline.ConditionsData{
				ConditionGroups: make([]conditions_kit.ConditionGroup, 0),
			},
		}).Model(),
		(&pipeline.GoBeginParallelTaskBlock{}).Model(),
		(&pipeline.GoWaitForAllInputsBlock{}).Model(),
		(&pipeline.ExecutableFunctionBlock{
			State: &pipeline.ExecutableFunction{
				Mapping:       make(map[string]script.JSONSchemaPropertiesValue, 0),
				Constants:     make(map[string]interface{}, 0),
				RetryTimeouts: make([]int, 0),
			},
		}).Model(),
		(&pipeline.TimerBlock{}).Model(),
		(&pipeline.GoNotificationBlock{
			State: &pipeline.NotificationData{
				People:          make([]string, 0),
				Emails:          make([]string, 0),
				UsersFromSchema: make(map[string]struct{}, 0),
			},
		}).Model(),
		(&pipeline.GoPlaceholderBlock{}).Model(),
		(&pipeline.GoStartBlock{}).Model(),
		(&pipeline.GoEndBlock{}).Model(),
	}
}

//nolint:goconst // common constants not needed
func (ae *Env) eriusFunctionTitle(id, currentTitle string) string {
	switch id {
	case IfBase:
		return IfBase
	case StringsIsEqualBase:
		return StringsIsEqualBase
	case ConnectorBase:
		return ConnectorBase
	case ForBase:
		return ForBase
	case "go_test_block":
		return "input"
	//nolint:goconst //ok
	case "approver":
		return "Согласование"
	case "sign":
		return "Подписание"
	case "servicedesk_application":
		return "Заявка Servicedesk"
	case "execution":
		return "Исполнение"
	case "form":
		return "Форма"
	case "start":
		return "Начало"
	case "end":
		return "Конец"
	case WaitForAllInputsBase:
		return "Параллельность конец"
	case BeginParallelTask:
		return "Параллельность начало"
	case pipeline.BlockPlaceholderID:
		if !ae.IncludePlaceholderBlock {
			return currentTitle
		}

		return "Задача"
	default:
		return currentTitle
	}
}
