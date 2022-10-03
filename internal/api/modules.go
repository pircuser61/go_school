package api

import (
	"encoding/json"
	"io"
	"net/http"

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
	BeginParallelTask    = "begin_parallel_task"
)

//nolint:gocyclo //its ok here
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

	eriusFunctions = append(eriusFunctions,
		(&pipeline.GoApproverBlock{}).Model(),
		(&pipeline.GoSdApplicationBlock{}).Model(),
		(&pipeline.GoExecutionBlock{}).Model(),
		(&pipeline.GoStartBlock{}).Model(),
		(&pipeline.GoEndBlock{}).Model(),
		(&pipeline.GoWaitForAllInputsBlock{}).Model(),
		(&pipeline.GoNotificationBlock{}).Model(),
		(&pipeline.IF{}).Model(),
		(&pipeline.GoBeginParallelTaskBlock{}).Model(),
		(&pipeline.ExecutableFunctionBlock{}).Model(),
		(&pipeline.GoFormBlock{}).Model(),
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
			Sockets:   []script.Socket{{Id: script.DefaultSocketID, Title: script.DefaultSocketTitle}},
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
		case "form":
			eriusFunctions[i].Title = "Форма"
		case "start":
			eriusFunctions[i].Title = "Начало"
		case "end":
			eriusFunctions[i].Title = "Конец"
		case WaitForAllImputsBase:
			eriusFunctions[i].Title = WaitForAllImputsBase
		case BeginParallelTask:
			eriusFunctions[i].Title = BeginParallelTask
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
				blocks[k].BlockType != script.TypePythonHTTP && blocks[k].BlockType != script.TypeGo &&
				blocks[k].BlockType != script.TypeExternal {
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

func (ae *APIEnv) ModuleUsage(w http.ResponseWriter, req *http.Request, moduleName string) {
	ctx, s := trace.StartSpan(req.Context(), "module_usage")
	defer s.End()

	log := logger.GetLogger(ctx)

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
			if f.Title == moduleName {
				usedBy = append(usedBy, entity.UsedBy{Name: pipe.Name, ID: pipe.ID})
				used = true

				break
			}
		}
	}

	err = sendResponse(w, http.StatusOK, entity.UsageResponse{Name: moduleName, Pipelines: usedBy, Used: used})
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

func (ae *APIEnv) ModuleRun(w http.ResponseWriter, req *http.Request, moduleName string) {
	ctx, s := trace.StartSpan(req.Context(), "module_run")
	defer s.End()

	log := logger.GetLogger(ctx)

	eriusFunctions, err := script.GetReadyFuncs(ctx, ae.ScriptManager, ae.HTTPClient)
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	block := script.FunctionModel{}

	for i := range eriusFunctions {
		if eriusFunctions[i].Title == moduleName {
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

	fb := pipeline.ExecutableFunctionBlock{
		Name:    block.Title,
		Input:   make(map[string]string),
		Output:  make(map[string]string),
		Sockets: []script.Socket{script.DefaultSocket},
		RunURL:  ae.FaaS + "function/%s",
	}

	for _, v := range block.Inputs {
		fb.Input[v.Name] = v.Name
	}

	for _, v := range block.Outputs {
		fb.Output[v.Name] = v.Name
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
