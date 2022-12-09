package functions

import (
	"context"
	"encoding/json"
	function_v1 "gitlab.services.mts.ru/jocasta/functions/pkg/proto/gen/function/v1"
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
	inputUnmarshalErr := json.Unmarshal([]byte(res.Function.Input), &input)
	if err != nil {
		return Function{}, inputUnmarshalErr
	}

	var output map[string]ParamMetadata
	outputUnmarshalErr := json.Unmarshal([]byte(res.Function.Output), &output)
	if err != nil {
		return Function{}, outputUnmarshalErr
	}

	/*var options map[string]interface{}
	optionsUnmarshalErr := json.Unmarshal([]byte(res.Function.Options), &options)
	if err != nil {
		return Function{}, optionsUnmarshalErr
	}*/

	versions := make([]Version, 0)

	for _, v := range res.Function.Versions {
		var versionInput map[string]interface{}
		versionInputUnmarshalErr := json.Unmarshal([]byte(v.Input), &versionInput)
		if err != nil {
			return Function{}, versionInputUnmarshalErr
		}

		var versionOutput map[string]interface{}
		versionOutputUnmarshalErr := json.Unmarshal([]byte(v.Output), &versionOutput)
		if err != nil {
			return Function{}, versionOutputUnmarshalErr
		}

		/*var versionOptions map[string]interface{}
		versionOptionsUnmarshalErr := json.Unmarshal([]byte(v.Options), &versionOptions)
		if err != nil {
			return Function{}, versionOptionsUnmarshalErr
		}*/

		versions = append(versions, Version{
			FunctionId:  v.FunctionId,
			VersionId:   v.VersionId,
			Description: v.Description,
			Version:     v.Version,
			Input:       versionInput,
			Output:      versionOutput,
			//Options:     versionoptions, //todo: deploy new functions api
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
