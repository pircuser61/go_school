package functions

import (
	"context"
	"encoding/json"
	function_v1 "gitlab.services.mts.ru/jocasta/functions/pkg/proto/gen/function/v1"
	"strconv"
)

func (s *Service) GetFunction(ctx context.Context, id string) (result Function, err error) {
	res, err := s.cli.GetFunctionById(ctx,
		&function_v1.GetFunctionRequest{
			FunctionId: id,
		},
	)

	if err != nil {
		return Function{}, err
	}

	var input map[string]ParamMetadata
	unquotedInput, unquoteInputErr := strconv.Unquote(res.Function.Input)
	if unquoteInputErr != nil {
		return Function{}, unquoteInputErr
	}
	inputUnmarshalErr := json.Unmarshal([]byte(unquotedInput), &input)
	if err != nil {
		return Function{}, inputUnmarshalErr
	}

	var output map[string]ParamMetadata
	unquotedOutput, unquoteOutputErr := strconv.Unquote(res.Function.Output)
	if unquoteOutputErr != nil {
		return Function{}, unquoteOutputErr
	}
	outputUnmarshalErr := json.Unmarshal([]byte(unquotedOutput), &output)
	if err != nil {
		return Function{}, outputUnmarshalErr
	}

	/*var options map[string]ParamMetadata
	unquotedOptions, unquoteOptionsErr := strconv.Unquote(res.Function.Options)
	if unquoteOptionsErr != nil {
		return Function{}, unquoteOptionsErr
	}
	optionsUnmarshalErr := json.Unmarshal([]byte(unquotedOptions), &options)
	if err != nil {
		return Function{}, optionsUnmarshalErr
	}*/

	versions := make([]Version, 0)

	for _, v := range res.Function.Versions {
		versions = append(versions, Version{
			FunctionId:  v.FunctionId,
			VersionId:   v.VersionId,
			Description: v.Description,
			Version:     v.Version,
			Input:       v.Input,
			Output:      v.Output,
			//Options:     v.Options, //todo: deploy new functions api
			CreatedAt: v.CreatedAt,
			DeletedAt: v.DeletedAt,
			UpdatedAt: v.UpdatedAt,
		})
	}

	return Function{
		FunctionId:  res.Function.FunctionId,
		VersionId:   res.Function.VersionId,
		Name:        res.Function.Name,
		Description: res.Function.Description,
		Version:     res.Function.Version,
		Uses:        res.Function.Uses,
		Input:       input,
		Output:      output,
		//Options:     options, // todo: deploy new functions api
		CreatedAt: res.Function.CreatedAt,
		DeletedAt: res.Function.DeletedAt,
		UpdatedAt: res.Function.UpdatedAt,
		Versions:  versions,
	}, nil
}
