package api

import (
	"net/http"

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

	eriusFunctions := eriusFunctions()

	eriusFunctionsResult := make([]script.FunctionModel, 0, len(eriusFunctions))

	for i := range eriusFunctions {
		eriusFunction := eriusFunctions[i]
		title := ae.eriusFunctionTitle(eriusFunction.ID, eriusFunction.Title)

		eriusFunctions[i].Title = title

		eriusFunctionsResult = append(eriusFunctionsResult, eriusFunctions[i])
	}

	eriusShapes, err := script.GetShapes()
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}

	err = sendResponse(w, http.StatusOK, entity.EriusFunctionList{Functions: eriusFunctionsResult, Shapes: eriusShapes})
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

func eriusFunctions() []script.FunctionModel {
	return []script.FunctionModel{
		(&pipeline.GoSdApplicationBlock{}).Model(),
		(&pipeline.GoFormBlock{}).Model(),
		(&pipeline.GoApproverBlock{}).Model(),
		(&pipeline.GoExecutionBlock{}).Model(),
		(&pipeline.GoSignBlock{}).Model(),
		(&pipeline.IF{}).Model(),
		(&pipeline.GoBeginParallelTaskBlock{}).Model(),
		(&pipeline.GoWaitForAllInputsBlock{}).Model(),
		(&pipeline.ExecutableFunctionBlock{}).Model(),
		(&pipeline.TimerBlock{}).Model(),
		(&pipeline.GoNotificationBlock{}).Model(),
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
		return WaitForAllInputsBase
	case BeginParallelTask:
		return BeginParallelTask
	case pipeline.BlockPlaceholderID:
		if !ae.IncludePlaceholderBlock {
			return currentTitle
		}

		return "Задача"
	default:
		return currentTitle
	}
}
