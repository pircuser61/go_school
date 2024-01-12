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

	versionNotFound := true
	versionIndex := 0
	for i, v := range function.Versions {
		if v.VersionId == versionID {
			versionNotFound = false
			versionIndex = i
			break
		}
	}

	if versionNotFound {
		return Function{}, fmt.Errorf("couldn't find function %s with version id %s", function.Name, versionID)
	}

	input, inputConvertErr := convertToParamMetadata(function.Versions[versionIndex].Input)
	if inputConvertErr != nil {
		return Function{}, inputConvertErr
	}

	output, outputConvertErr := convertToParamMetadata(function.Versions[versionIndex].Output)
	if outputConvertErr != nil {
		return Function{}, outputConvertErr
	}

	var options Options
	optionsUnmarshalErr := json.Unmarshal([]byte(function.Versions[versionIndex].Options), &options)
	if err != nil {
		return Function{}, optionsUnmarshalErr
	}

	return Function{
		Name:        function.Name,
		FunctionId:  function.Versions[versionIndex].FunctionId,
		VersionId:   function.Versions[versionIndex].VersionId,
		Description: function.Versions[versionIndex].Description,
		Version:     function.Versions[versionIndex].Version,
		Uses:        function.Uses,
		Input:       input,
		Output:      output,
		Options:     options,
		Contracts:   function.Versions[versionIndex].Contracts,
		CreatedAt:   function.Versions[versionIndex].CreatedAt,
		DeletedAt:   function.Versions[versionIndex].DeletedAt,
		UpdatedAt:   function.Versions[versionIndex].UpdatedAt,
		Versions:    function.Versions,
	}, nil
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
			FunctionId:  v.FunctionId,
			VersionId:   v.VersionId,
			Description: v.Description,
			Version:     v.Version,
			Input:       v.Input,
			Output:      v.Output,
			Options:     v.Options,
			Contracts:   v.Contracts,
			CreatedAt:   v.CreatedAt,
			DeletedAt:   v.DeletedAt,
			UpdatedAt:   v.UpdatedAt,
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
		Options:     options,
		Contracts:   res.Function.Contracts,
		CreatedAt:   res.Function.CreatedAt,
		DeletedAt:   res.Function.DeletedAt,
		UpdatedAt:   res.Function.UpdatedAt,
		Versions:    versions,
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
