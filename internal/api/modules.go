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
			State: pipeline.NewFormState(),
		}).Model(),
		(&pipeline.GoApproverBlock{
			State: pipeline.NewApproverState(),
		}).Model(),
		(&pipeline.GoExecutionBlock{
			State: pipeline.NewExecutionState(),
		}).Model(),
		(&pipeline.GoSignBlock{
			State: pipeline.NewSignState(),
		}).Model(),
		(&pipeline.IF{
			State: pipeline.NewIfState(),
		}).Model(),
		(&pipeline.GoBeginParallelTaskBlock{}).Model(),
		(&pipeline.GoWaitForAllInputsBlock{}).Model(),
		(&pipeline.ExecutableFunctionBlock{
			State: pipeline.NewExecutableFunctionState(),
		}).Model(),
		(&pipeline.TimerBlock{}).Model(),
		(&pipeline.GoNotificationBlock{
			State: pipeline.NewNotificationState(),
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
