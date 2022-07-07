package handlers

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

const (
	WaitForAllImputsBase = "wait_for_all_inputs"
	IfBase               = "if"
	ConnectorBase        = "connector"
	ForBase              = "for"
	StringsIsEqualBase   = "strings_is_equal"
)

// GetModules godoc
// @Summary Get list of modules
// @Description Список блоков
// @Tags modules
// @ID      get-modules
// @Produce json
// @Success 200 {object} httpResponse{data=entity.EriusFunctionList}
// @Failure 400 {object} httpError
// @Failure 500 {object} httpError
// @Router /modules [get]
// nolint:gocyclo // future rewrite
func (ae *APIEnv) GetModules(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "list_modules")
	defer s.End()

	log := logger.GetLogger(ctx)

	eriusFunctions, err := script.GetReadyFuncs(ctx, ae.ScriptManager, ae.HTTPClient)
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	approverBlock := &pipeline.GoApproverBlock{}

	sdApplicationBlock := &pipeline.GoSdApplicationBlock{}

	executionBlock := &pipeline.GoExecutionBlock{}

	startBlock := &pipeline.GoStartBlock{}

	endBlock := &pipeline.GoEndBlock{}

	waitForAllInputs := &pipeline.GoWaitForAllInputsBlock{}

	notificationBlock := &pipeline.GoNotificationBlock{}

	ifBlock := &pipeline.IF{}

	eriusFunctions = append(eriusFunctions,
		script.IfState.Model(),
		script.Input.Model(),
		script.Equal.Model(),
		script.Connector.Model(),
		script.ForState.Model(),
		approverBlock.Model(),
		sdApplicationBlock.Model(),
		executionBlock.Model(),
		startBlock.Model(),
		endBlock.Model(),
		waitForAllInputs.Model(),
		notificationBlock.Model(),
		ifBlock.Model(),
	)

	scenarios, err := ae.DB.GetExecutableScenarios(ctx)
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	for i := range scenarios {
		scenario := &scenarios[i]
		b := script.FunctionModel{
			BlockType: script.TypeScenario,
			Title:     scenario.Name,
			Inputs:    make([]script.FunctionValueModel, 0),
			Outputs:   make([]script.FunctionValueModel, 0),
			ShapeType: script.ShapeScenario,
			Sockets:   []string{pipeline.DefaultSocket},
		}

		for _, v := range scenario.Input {
			b.Inputs = append(b.Inputs, script.FunctionValueModel{
				Name: v.Name,
				Type: v.Type,
			})
		}

		for _, v := range scenario.Output {
			b.Outputs = append(b.Outputs, script.FunctionValueModel{
				Name: v.Name,
				Type: v.Type,
			})
		}

		eriusFunctions = append(eriusFunctions, b)
	}

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
		case "approver":
			eriusFunctions[i].Title = "Согласование"
		case "servicedesk_application":
			eriusFunctions[i].Title = "Заявка Servicedesk"
		case "execution":
			eriusFunctions[i].Title = "Исполнение"
		case "start":
			eriusFunctions[i].Title = "Начало"
		case "end":
			eriusFunctions[i].Title = "Конец"
		case WaitForAllImputsBase:
			eriusFunctions[i].Title = WaitForAllImputsBase
		}
	}

	eriusShapes, err := script.GetShapes()
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = sendResponse(w, http.StatusOK, entity.EriusFunctionList{Functions: eriusFunctions, Shapes: eriusShapes})
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

// AllModulesUsage godoc
// @Summary Get list of modules usage
// @Description Блоки и сценарии, в которых они используются
// @Tags modules
// @ID      modules-usage
// @Produce json
// @success 200 {object} httpResponse{data=entity.AllUsageResponse}
// @Failure 400 {object} httpError
// @Failure 500 {object} httpError
// @Router /modules/usage [get]
func (ae *APIEnv) AllModulesUsage(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "all_modules_usage")
	defer s.End()

	log := logger.GetLogger(ctx)

	scenarios, err := ae.DB.GetWorkedVersions(ctx)
	if err != nil {
		e := ModuleUsageError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	moduleUsageMap := make(map[string]map[string]struct{})

	for i := range scenarios {
		blocks := scenarios[i].Pipeline.Blocks
		for k := range blocks {
			if blocks[k].BlockType != script.TypePython3 && blocks[k].BlockType != script.TypePythonFlask &&
				blocks[k].BlockType != script.TypePythonHTTP && blocks[k].BlockType != script.TypeGo {
				continue
			}

			name := blocks[k].Title
			if _, ok := moduleUsageMap[name]; !ok {
				moduleUsageMap[name] = make(map[string]struct{})
			}

			moduleUsageMap[name][scenarios[i].Name] = struct{}{}
		}
	}

	resp := make(map[string][]string)

	for module, pipes := range moduleUsageMap {
		p := make([]string, 0, len(pipes))
		for n := range pipes {
			p = append(p, n)
		}

		resp[module] = p
	}

	err = sendResponse(w, http.StatusOK, entity.AllUsageResponse{Functions: resp})
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

// ModuleUsage godoc
// @Summary Usage of module in pipelines
// @Description Сценарии, в которых используется блок
// @Tags modules
// @ID      module-usage
// @Produce json
// @Param moduleName path string true "module name"
// @Success 200 {object} httpResponse{data=entity.UsageResponse}
// @Failure 400 {object} httpError
// @Failure 500 {object} httpError
// @Router /modules/{moduleName}/usage [get]
func (ae *APIEnv) ModuleUsage(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "module_usage")
	defer s.End()

	log := logger.GetLogger(ctx)

	name := chi.URLParam(req, "moduleName")

	allWorked, err := ae.DB.GetWorkedVersions(ctx)
	if err != nil {
		e := ModuleUsageError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	usedBy := make([]entity.UsedBy, 0)
	used := false

	for i := range allWorked {
		pipe := &allWorked[i]
		for j := range pipe.Pipeline.Blocks {
			f := pipe.Pipeline.Blocks[j]
			if f.Title == name {
				usedBy = append(usedBy, entity.UsedBy{Name: pipe.Name, ID: pipe.ID})
				used = true

				break
			}
		}
	}

	err = sendResponse(w, http.StatusOK, entity.UsageResponse{Name: name, Pipelines: usedBy, Used: used})
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

// ModuleRun godoc
// @Summary Run Module By Name
// @Description Запустить блок
// @Tags modules
// @ID      module-usage-by-name
// @Produce json
// @Param moduleName path string true "module name"
// @Success 200 {object} httpResponse{data=entity.UsageResponse}
// @Failure 400 {object} httpError
// @Failure 500 {object} httpError
// @Router /modules/{moduleName} [post]
func (ae *APIEnv) ModuleRun(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "module_run")
	defer s.End()

	log := logger.GetLogger(ctx)

	name := chi.URLParam(req, "moduleName")

	eriusFunctions, err := script.GetReadyFuncs(ctx, ae.ScriptManager, ae.HTTPClient)
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	block := script.FunctionModel{}

	for i := range eriusFunctions {
		if eriusFunctions[i].Title == name {
			block = eriusFunctions[i]

			break
		}
	}

	if block.Title == "" {
		e := ModuleUsageError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	fb := pipeline.FunctionBlock{
		Name:           block.Title,
		FunctionName:   block.Title,
		FunctionInput:  make(map[string]string),
		FunctionOutput: make(map[string]string),
		Nexts:          map[string][]string{pipeline.DefaultSocket: []string{}},
		RunURL:         ae.FaaS + "function/%s",
	}

	for _, v := range block.Inputs {
		fb.FunctionInput[v.Name] = v.Name
	}

	for _, v := range block.Outputs {
		fb.FunctionOutput[v.Name] = v.Name
	}

	vs := store.NewStore()

	b, err := io.ReadAll(req.Body)
	defer req.Body.Close()

	if err != nil {
		e := RequestReadError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	pipelineVars := make(map[string]interface{})

	if len(b) != 0 {
		err = json.Unmarshal(b, &pipelineVars)
		if err != nil {
			e := PipelineRunError
			log.Error(e.errorMessage(err))
			_ = e.sendError(w)

			return
		}

		for key, value := range pipelineVars {
			vs.SetValue(key, value)
		}
	}

	result, err := fb.RunOnly(ctx, vs)
	if err != nil {
		e := PipelineRunError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = sendResponse(w, http.StatusOK, result)
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}
