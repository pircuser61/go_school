package handlers

import (
	"context"
	"net/http"

	"github.com/go-chi/chi"
	"gitlab.services.mts.ru/erius/pipeliner/internal/entity"
	"gitlab.services.mts.ru/erius/pipeliner/internal/integration"
	"gitlab.services.mts.ru/erius/pipeliner/internal/script"
	"go.opencensus.io/trace"
)

func (ae APIEnv) GetModules(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(context.Background(), "list_modules")
	defer s.End()

	eriusFunctions, err := script.GetReadyFuncs(ctx, ae.ScriptManager)
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
		integration.NewNGSASendIntegration(ae.DB, 3, "").Model())

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

func (ae APIEnv) AllModulesUsage(w http.ResponseWriter, req *http.Request) {
	c, s := trace.StartSpan(context.Background(), "all_modules_usage")
	defer s.End()

	scenarios, err := ae.DB.GetWorkedVersions(c)
	if err != nil {
		e := ModuleUsageError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	moduleUsageMap := make(map[string]map[string]struct{})
	for i, _ := range scenarios {
		for j, _ := range scenarios[i].Pipeline.Blocks {
			if scenarios[i].Pipeline.Blocks[j].BlockType != script.TypePython3 {
				continue
			}
			name := scenarios[i].Pipeline.Blocks[j].Title
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

func (ae APIEnv) ModuleUsage(w http.ResponseWriter, req *http.Request) {
	c, s := trace.StartSpan(context.Background(), "module_usage")
	defer s.End()

	name := chi.URLParam(req, "moduleName")

	allWorked, err := ae.DB.GetWorkedVersions(c)
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
