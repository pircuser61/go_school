package handlers

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"gitlab.services.mts.ru/erius/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/erius/pipeliner/internal/store"

	"github.com/go-chi/chi"
	"gitlab.services.mts.ru/erius/pipeliner/internal/entity"
	"gitlab.services.mts.ru/erius/pipeliner/internal/integration"
	"gitlab.services.mts.ru/erius/pipeliner/internal/script"
	"go.opencensus.io/trace"
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
// @Router /modules/ [get]
func (ae *APIEnv) GetModules(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "list_modules")
	defer s.End()

	eriusFunctions, err := script.GetReadyFuncs(ctx, ae.ScriptManager, ae.HTTPClient)
	if err != nil {
		e := UnknownError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	eriusFunctions = append(eriusFunctions,
		script.IfState.Model(),
		script.Input.Model(),
		script.Equal.Model(),
		script.Vars.Model(),
		script.Connector.Model(),
		script.ForState.Model(),
		integration.NewNGSASendIntegration(ae.DB).Model(),
		integration.NewRemedySendCreateMI(ae.Remedy, ae.HTTPClient).Model(),
		integration.NewRemedySendCreateWork(ae.Remedy, ae.HTTPClient).Model(),
		integration.NewRemedySendCreateProblem(ae.Remedy, ae.HTTPClient).Model(),
		integration.NewRemedySendUpdateMI(ae.Remedy, ae.HTTPClient).Model(),
		integration.NewRemedySendUpdateWork(ae.Remedy, ae.HTTPClient).Model(),
		integration.NewRemedySendUpdateProblem(ae.Remedy, ae.HTTPClient).Model())

	scenarios, err := ae.DB.GetExecutableScenarios(ctx)
	if err != nil {
		e := UnknownError
		ae.Logger.Error(e.errorMessage(err))
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
			NextFuncs: []string{script.Next},
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
		v := eriusFunctions[i]
		id := v.Title + v.BlockType
		v.ID = id
		eriusFunctions[i] = v
	}

	eriusShapes, err := script.GetShapes()
	if err != nil {
		e := UnknownError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = sendResponse(w, http.StatusOK, entity.EriusFunctionList{Functions: eriusFunctions, Shapes: eriusShapes})
	if err != nil {
		e := UnknownError
		ae.Logger.Error(e.errorMessage(err))
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

	scenarios, err := ae.DB.GetWorkedVersions(ctx)
	if err != nil {
		e := ModuleUsageError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	moduleUsageMap := make(map[string]map[string]struct{})

	for i := range scenarios {
		blocks := scenarios[i].Pipeline.Blocks
		for k := range blocks {
			if blocks[k].BlockType != script.TypePython3 {
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
		ae.Logger.Error(e.errorMessage(err))
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

	name := chi.URLParam(req, "moduleName")

	allWorked, err := ae.DB.GetWorkedVersions(ctx)
	if err != nil {
		e := ModuleUsageError
		ae.Logger.Error(e.errorMessage(err))
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
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

// ModuleRun godoc
// @Summary Run Module By Name
// @Description Запустить блок
// @Tags modules
// @ID      module-usage
// @Produce json
// @Param moduleName path string true "module name"
// @Success 200 {object} httpResponse{data=entity.UsageResponse}
// @Failure 400 {object} httpError
// @Failure 500 {object} httpError
// @Router /modules/{moduleName} [post]
func (ae *APIEnv) ModuleRun(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "module_run")
	defer s.End()

	name := chi.URLParam(req, "moduleName")

	eriusFunctions, err := script.GetReadyFuncs(ctx, ae.ScriptManager, ae.HTTPClient)
	if err != nil {
		e := UnknownError
		ae.Logger.Error(e.errorMessage(err))
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
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	fb := pipeline.FunctionBlock{
		Name:           block.Title,
		FunctionName:   block.Title,
		FunctionInput:  make(map[string]string),
		FunctionOutput: make(map[string]string),
		NextStep:       "",
		RunURL:         ae.FaaS + "function/%s",
	}

	for _, v := range block.Inputs {
		fb.FunctionInput[v.Name] = v.Name
	}

	for _, v := range block.Outputs {
		fb.FunctionOutput[v.Name] = v.Name
	}

	vs := store.NewStore()

	b, err := ioutil.ReadAll(req.Body)
	defer req.Body.Close()

	if err != nil {
		e := RequestReadError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	pipelineVars := make(map[string]interface{})

	if len(b) != 0 {
		err = json.Unmarshal(b, &pipelineVars)
		if err != nil {
			e := PipelineRunError
			ae.Logger.Error(e.errorMessage(err))
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
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = sendResponse(w, http.StatusOK, result)
	if err != nil {
		e := UnknownError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}
