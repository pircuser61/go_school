package functions

import (
	"context"
	"encoding/json"
	"fmt"

	function_v1 "gitlab.services.mts.ru/jocasta/functions/pkg/proto/gen/function/v1"
)

func (s *Service) GetFunctionVersion(ctx context.Context, functionID, versionID string) (result Function, err error) {
	function, err := s.GetFunction(ctx, functionID)
	if err != nil {
		return Function{}, err
	}

	for index := range function.Versions {
		if function.Versions[index].VersionID != versionID {
			continue
		}

		input, inputConvertErr := convertToParamMetadata(function.Versions[index].Input)
		if inputConvertErr != nil {
			return Function{}, inputConvertErr
		}

		output, outputConvertErr := convertToParamMetadata(function.Versions[index].Output)
		if outputConvertErr != nil {
			return Function{}, outputConvertErr
		}

		var options Options

		optionsUnmarshalErr := json.Unmarshal([]byte(function.Versions[index].Options), &options)
		if err != nil {
			return Function{}, optionsUnmarshalErr
		}

		return Function{
			Name:          function.Name,
			FunctionID:    function.Versions[index].FunctionID,
			VersionID:     function.Versions[index].VersionID,
			Description:   function.Versions[index].Description,
			Version:       function.Versions[index].Version,
			Uses:          function.Uses,
			Input:         input,
			RequiredInput: function.Versions[index].RequiredInput,
			Output:        output,
			Options:       options,
			Contracts:     function.Versions[index].Contracts,
			CreatedAt:     function.Versions[index].CreatedAt,
			DeletedAt:     function.Versions[index].DeletedAt,
			UpdatedAt:     function.Versions[index].UpdatedAt,
			Versions:      function.Versions,
		}, nil
	}

	return Function{}, fmt.Errorf("couldn't find function with id %s with version id %s", functionID, versionID)
}

func (s *Service) GetFunction(ctx context.Context, id string) (result Function, err error) {
	res, err := s.cli.GetFunctionById(ctx,
		&function_v1.GetFunctionRequest{
			FunctionId: id,
		},
	)
	if err != nil {
		return Function{}, err
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
