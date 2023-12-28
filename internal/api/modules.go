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

//nolint:gocyclo //its ok here
func (ae *APIEnv) GetModules(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "list_modules")
	defer s.End()

	log := logger.GetLogger(ctx)

	eriusFunctions := []script.FunctionModel{
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

	eriusFunctionsReturn := make([]script.FunctionModel, 0, len(eriusFunctions))

	for i := range eriusFunctions {
		switch eriusFunctions[i].ID {
		case IfBase:
			eriusFunctions[i].Title = IfBase
		case StringsIsEqualBase:
			eriusFunctions[i].Title = StringsIsEqualBase
		case ConnectorBase:
			eriusFunctions[i].Title = ConnectorBase
		case ForBase:
			eriusFunctions[i].Title = ForBase
		case "go_test_block":
			eriusFunctions[i].Title = "input"
		//nolint:goconst //ok
		case "approver":
			eriusFunctions[i].Title = "Согласование"
		case "sign":
			eriusFunctions[i].Title = "Подписание"
		case "servicedesk_application":
			eriusFunctions[i].Title = "Заявка Servicedesk"
		case "execution":
			eriusFunctions[i].Title = "Исполнение"
		case "form":
			eriusFunctions[i].Title = "Форма"
		case "start":
			eriusFunctions[i].Title = "Начало"
		case "end":
			eriusFunctions[i].Title = "Конец"
		case WaitForAllInputsBase:
			eriusFunctions[i].Title = WaitForAllInputsBase
		case BeginParallelTask:
			eriusFunctions[i].Title = BeginParallelTask
		case pipeline.BlockPlaceholderID:
			if !ae.IncludePlaceholderBlock {
				continue
			}
			eriusFunctions[i].Title = "Задача"
		case "":
		}
		eriusFunctionsReturn = append(eriusFunctionsReturn, eriusFunctions[i])
	}

	eriusShapes, err := script.GetShapes()
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = sendResponse(w, http.StatusOK, entity.EriusFunctionList{Functions: eriusFunctionsReturn, Shapes: eriusShapes})
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}
