package functions

import (
	c "context"
	"encoding/json"
	"fmt"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	function "gitlab.services.mts.ru/jocasta/functions/pkg/proto/gen/function/v1"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

func (s *service) GetFunctionVersion(ctx c.Context, functionID, versionID string) (res Function, err error) {
	fn, err := s.GetFunction(ctx, functionID)
	if err != nil {
		return Function{}, err
	}

	for index := range fn.Versions {
		if fn.Versions[index].VersionID != versionID {
			continue
		}

		input, inputConvertErr := convertToParamMetadata(fn.Versions[index].Input)
		if inputConvertErr != nil {
			return Function{}, inputConvertErr
		}

		output, outputConvertErr := convertToParamMetadata(fn.Versions[index].Output)
		if outputConvertErr != nil {
			return Function{}, outputConvertErr
		}

		var options Options

		optionsUnmarshalErr := json.Unmarshal([]byte(fn.Versions[index].Options), &options)
		if err != nil {
			return Function{}, optionsUnmarshalErr
		}

		return Function{
			Name:          fn.Name,
			FunctionID:    fn.Versions[index].FunctionID,
			VersionID:     fn.Versions[index].VersionID,
			Description:   fn.Versions[index].Description,
			Version:       fn.Versions[index].Version,
			Uses:          fn.Uses,
			Input:         input,
			RequiredInput: fn.Versions[index].RequiredInput,
			Output:        output,
			Options:       options,
			Contracts:     fn.Versions[index].Contracts,
			CreatedAt:     fn.Versions[index].CreatedAt,
			DeletedAt:     fn.Versions[index].DeletedAt,
			UpdatedAt:     fn.Versions[index].UpdatedAt,
			Versions:      fn.Versions,
		}, nil
	}

	return Function{}, fmt.Errorf("couldn't find function with id %s with version id %s", functionID, versionID)
}

func (s *service) GetFunction(ctx c.Context, id string) (result Function, err error) {
	ctxLocal, span := trace.StartSpan(ctx, "functions.get_function")
	defer span.End()

	log := logger.GetLogger(ctxLocal).
		WithField("traceID", span.SpanContext().TraceID.String()).WithField("transport", "GRPC")

	ctxLocal = script.MakeContextWithRetryCnt(ctxLocal)

	res, err := s.cli.GetFunctionById(ctxLocal,
		&function.GetFunctionRequest{
			FunctionId: id,
		},
	)

	attempt := script.GetRetryCnt(ctxLocal)

	if err != nil {
		log.Warning("Pipeliner failed to connect to functions. Exceeded max retry count: ", attempt)

		return Function{}, err
	}

	if attempt > 0 {
		log.Warning("Pipeliner successfully reconnected to functions: ", attempt)
	}

	input, inputConvertErr := convertToParamMetadata(res.Function.Input)
	if inputConvertErr != nil {
		return Function{}, inputConvertErr
	}

	output, outputConvertErr := convertToParamMetadata(res.Function.Output)
	if outputConvertErr != nil {
		return Function{}, outputConvertErr
	}

	var options Options

	optionsUnmarshalErr := json.Unmarshal([]byte(res.Function.Options), &options)
	if err != nil {
		return Function{}, optionsUnmarshalErr
	}

	versions := make([]Version, 0)

	for _, v := range res.Function.Versions {
		versions = append(versions, Version{
			FunctionID:    v.FunctionId,
			VersionID:     v.VersionId,
			Description:   v.Description,
			Version:       v.Version,
			Input:         v.Input,
			RequiredInput: v.RequiredInput,
			Output:        v.Output,
			Options:       v.Options,
			Contracts:     v.Contracts,
			CreatedAt:     v.CreatedAt,
			DeletedAt:     v.DeletedAt,
			UpdatedAt:     v.UpdatedAt,
		})
	}

	return Function{
		FunctionID:    res.Function.FunctionId,
		VersionID:     res.Function.VersionId,
		Name:          res.Function.Name,
		Description:   res.Function.Description,
		Version:       res.Function.Version,
		Uses:          res.Function.Uses,
		Input:         input,
		RequiredInput: res.Function.RequiredInput,
		Output:        output,
		Options:       options,
		Contracts:     res.Function.Contracts,
		CreatedAt:     res.Function.CreatedAt,
		DeletedAt:     res.Function.DeletedAt,
		UpdatedAt:     res.Function.UpdatedAt,
		Versions:      versions,
	}, nil
}

func convertToParamMetadata(source string) (result map[string]ParamMetadata, err error) {
	unmarshalErr := json.Unmarshal([]byte(source), &result)
	if unmarshalErr != nil {
		err = unmarshalErr

		return nil, err
	}

	return result, nil
}
