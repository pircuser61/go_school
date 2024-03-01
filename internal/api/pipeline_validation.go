package api

import (
	"context"
	"encoding/json"
	"strings"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

const (
	objectType = "object"
	arrayType  = "array"
	stringType = "string"

	conditionGroupsField = "conditionGroups"
	conditionsField      = "conditions"
	leftOperand          = "leftOperand"
	rightOperand         = "rightOperand"
	variableRefField     = "variableRef"
)

func (ae *Env) validatePipeline(ctx context.Context, pipeline *entity.EriusScenario) (valid bool, textErr string) {
	ok := validateMapping(pipeline.Pipeline.Blocks)
	if !ok {
		return false, entity.PipelineValidateError
	}

	return pipeline.Pipeline.Blocks.Validate(ctx, ae.ServiceDesc)
}

func validateMapping(bt entity.BlocksType) bool {
	isValid := true

	for blockName, block := range bt {
		switch block.TypeID {
		case pipeline.BlockExecutableFunctionID:
			if !validateFunctionBlock(bt, block, blockName) {
				isValid = false
			}
		case pipeline.BlockGoApproverID:
			if !validateApproverBlock(bt, block, blockName) {
				isValid = false
			}
		case pipeline.BlockGoExecutionID:
			if !validateExecutionBlock(bt, block, blockName) {
				isValid = false
			}
		case pipeline.BlockGoFormID:
			if !validateFormBlock(bt, block, blockName) {
				isValid = false
			}
		case pipeline.BlockGoSignID:
			if !validateSignBlock(bt, block, blockName) {
				isValid = false
			}
		case pipeline.BlockGoIfID:
			if !validateIfBlock(bt, block, blockName) {
				isValid = false
			}
		case pipeline.BlockGoNotificationID:
			if !validateNotificationBlock(bt, block, blockName) {
				isValid = false
			}
		}
	}

	return isValid
}

func validateFunctionBlock(bt entity.BlocksType, block entity.EriusFunc, blockName string) bool {
	var function script.ExecutableFunctionParams
	err := json.Unmarshal(block.Params, &function)
	if err != nil {
		return false
	}

	if !validateProperties(function.Mapping, bt, blockName) {
		var marshaledFunction []byte
		marshaledFunction, err = json.Marshal(function)
		if err != nil {
			return false
		}

		block.Params = marshaledFunction
		bt[blockName] = block

		return false
	}

	return true
}

func validateApproverBlock(bt entity.BlocksType, block entity.EriusFunc, blockName string) bool {
	var blockApprover script.ApproverParams
	err := json.Unmarshal(block.Params, &blockApprover)
	if err != nil {
		return false
	}

	isValid := true

	if blockApprover.Type == script.ApproverTypeFromSchema {
		var validVars []string
		approverVars := strings.Split(blockApprover.Approver, ";")
		for _, approverVar := range approverVars {
			schema := &script.JSONSchemaPropertiesValue{
				Type:       objectType,
				Properties: people.GetSsoPersonSchemaProperties(),
				Value:      approverVar,
			}

			if !isPathValid(bt, schema, blockName) {
				isValid = false
				continue
			}

			validVars = append(validVars, approverVar)
		}

		if !isValid {
			blockApprover.Approver = strings.Join(validVars, ";")
		}
	}

	if blockApprover.ApproversGroupIDPath != nil && *blockApprover.ApproversGroupIDPath != "" {
		schema := &script.JSONSchemaPropertiesValue{
			Type:  stringType,
			Value: *blockApprover.ApproversGroupIDPath,
		}

		if !isPathValid(bt, schema, blockName) {
			isValid = false
			blockApprover.ApproversGroupIDPath = nil
		}
	}

	if !isValid {
		var marshaledApproverBlock []byte
		marshaledApproverBlock, err = json.Marshal(blockApprover)
		if err != nil {
			return false
		}

		block.Params = marshaledApproverBlock
		bt[blockName] = block
	}

	return isValid
}

func validateExecutionBlock(bt entity.BlocksType, block entity.EriusFunc, blockName string) bool {
	var blockExecution script.ExecutionParams
	err := json.Unmarshal(block.Params, &blockExecution)
	if err != nil {
		return false
	}

	isValid := true

	if blockExecution.Type == script.ExecutionTypeFromSchema {
		var validVars []string

		executorVars := strings.Split(blockExecution.Executors, ";")
		for _, executorVar := range executorVars {
			schema := &script.JSONSchemaPropertiesValue{
				Type:       objectType,
				Properties: people.GetSsoPersonSchemaProperties(),
				Value:      executorVar,
			}

			if !isPathValid(bt, schema, blockName) {
				isValid = false
				continue
			}

			validVars = append(validVars, executorVar)
		}

		if !isValid {
			blockExecution.Executors = strings.Join(validVars, ";")
		}
	}

	if blockExecution.ExecutorsGroupIDPath != nil && *blockExecution.ExecutorsGroupIDPath != "" {
		schema := &script.JSONSchemaPropertiesValue{
			Type:  stringType,
			Value: *blockExecution.ExecutorsGroupIDPath,
		}

		if !isPathValid(bt, schema, blockName) {
			isValid = false
			blockExecution.ExecutorsGroupIDPath = nil
		}
	}

	if !isValid {
		var marshaledExecutionBlock []byte
		marshaledExecutionBlock, err = json.Marshal(blockExecution)
		if err != nil {
			return false
		}

		block.Params = marshaledExecutionBlock
		bt[blockName] = block
	}

	return isValid
}

func validateFormBlock(bt entity.BlocksType, block entity.EriusFunc, blockName string) bool {
	var blockForm script.FormParams
	err := json.Unmarshal(block.Params, &blockForm)
	if err != nil {
		return false
	}

	isValid := true
	if !validateProperties(blockForm.Mapping, bt, blockName) {
		isValid = false
	}

	objectSchema := &script.JSONSchemaPropertiesValue{
		Type:  objectType,
		Value: blockForm.FullFormMapping,
	}

	if !isPathValid(bt, objectSchema, blockName) {
		isValid = false
		blockForm.FullFormMapping = ""
	}

	if blockForm.FormExecutorType == script.FormExecutorTypeFromSchema {
		var validVars []string

		executorVars := strings.Split(blockForm.Executor, ";")
		for _, executorVar := range executorVars {
			schema := &script.JSONSchemaPropertiesValue{
				Type:       objectType,
				Properties: people.GetSsoPersonSchemaProperties(),
				Value:      executorVar,
			}

			if !isPathValid(bt, schema, blockName) {
				isValid = false
				continue
			}

			validVars = append(validVars, executorVar)
		}

		if !isValid {
			blockForm.Executor = strings.Join(validVars, ";")
		}
	}

	if blockForm.FormGroupIDPath != nil && *blockForm.FormGroupIDPath != "" {
		schema := &script.JSONSchemaPropertiesValue{
			Type:  stringType,
			Value: *blockForm.FormGroupIDPath,
		}

		if !isPathValid(bt, schema, blockName) {
			isValid = false
			blockForm.FormGroupIDPath = nil
		}
	}

	if !isValid {
		var marshaledForm []byte
		marshaledForm, err = json.Marshal(blockForm)
		if err != nil {
			return false
		}

		block.Params = marshaledForm
		bt[blockName] = block
	}

	return isValid
}

func validateSignBlock(bt entity.BlocksType, block entity.EriusFunc, blockName string) bool {
	var blockSign script.SignParams
	err := json.Unmarshal(block.Params, &blockSign)
	if err != nil {
		return false
	}

	isValid := true

	if blockSign.Type == script.SignerTypeFromSchema {
		var validVars []string

		signerVars := strings.Split(blockSign.Signer, ";")
		for _, signerVar := range signerVars {
			schema := &script.JSONSchemaPropertiesValue{
				Type:       objectType,
				Properties: people.GetSsoPersonSchemaProperties(),
				Value:      signerVar,
			}

			if !isPathValid(bt, schema, blockName) {
				isValid = false
				continue
			}

			validVars = append(validVars, signerVar)
		}

		if !isValid {
			blockSign.Signer = strings.Join(validVars, ";")
		}
	}

	if blockSign.SignerGroupIDPath != "" {
		schema := &script.JSONSchemaPropertiesValue{
			Type:  stringType,
			Value: blockSign.SignerGroupIDPath,
		}

		if !isPathValid(bt, schema, blockName) {
			isValid = false
			blockSign.SignerGroupIDPath = ""
		}
	}

	schema := &script.JSONSchemaPropertiesValue{
		Type:  stringType,
		Value: blockSign.SigningParamsPaths.SNILS,
	}

	if !isPathValid(bt, schema, blockName) {
		isValid = false
		blockSign.SigningParamsPaths.SNILS = ""
	}

	schema.Value = blockSign.SigningParamsPaths.INN

	if !isPathValid(bt, schema, blockName) {
		isValid = false
		blockSign.SigningParamsPaths.INN = ""
	}

	var validFiles []string

	for _, fileRef := range blockSign.SigningParamsPaths.Files {
		fileSchema := &script.JSONSchemaPropertiesValue{
			Type: objectType,
			Properties: map[string]script.JSONSchemaPropertiesValue{
				"file_id": {
					Type: stringType,
				},
				"external_link": {
					Type: stringType,
				},
			},
			Value: fileRef,
		}

		if !isPathValid(bt, fileSchema, blockName) {
			isValid = false
			continue
		}

		validFiles = append(validFiles, fileRef)
	}

	if len(validFiles) != len(blockSign.SigningParamsPaths.Files) {
		blockSign.SigningParamsPaths.Files = validFiles
	}

	if blockSign.SigningParamsPaths.SNILS == "" || blockSign.SigningParamsPaths.INN == "" {
		isValid = false
	}

	if !isValid {
		var marshaledSignBlock []byte
		marshaledSignBlock, err = json.Marshal(blockSign)
		if err != nil {
			return false
		}

		block.Params = marshaledSignBlock
		bt[blockName] = block
	}

	return isValid
}

func validateIfBlock(bt entity.BlocksType, block entity.EriusFunc, blockName string) bool {
	var blockIf map[string]interface{}
	err := json.Unmarshal(block.Params, &blockIf)
	if err != nil {
		return false
	}

	conditionGroupsInterface, ok := blockIf[conditionGroupsField]
	if !ok {
		return false
	}

	conditionGroups, ok := conditionGroupsInterface.([]interface{})
	if !ok {
		return false
	}

	isValid := true

	for _, groupInterface := range conditionGroups {
		var group map[string]interface{}
		group, ok = groupInterface.(map[string]interface{})
		if !ok {
			continue
		}

		var conditionsInterface interface{}
		conditionsInterface, ok = group[conditionsField]
		if !ok {
			continue
		}

		var conditions []interface{}
		conditions, ok = conditionsInterface.([]interface{})
		if !ok {
			continue
		}

		for _, conditionInterface := range conditions {
			var condition map[string]interface{}
			condition, ok = conditionInterface.(map[string]interface{})
			if !ok {
				continue
			}

			var operandInterface interface{}
			if operandInterface, ok = condition[leftOperand]; ok {
				if !validateOperand(bt, operandInterface, blockName) {
					isValid = false
				}
			}

			if operandInterface, ok = condition[rightOperand]; ok {
				if !validateOperand(bt, operandInterface, blockName) {
					isValid = false
				}
			}
		}
	}

	if !isValid {
		var marshaledIfBlock []byte
		marshaledIfBlock, err = json.Marshal(blockIf)
		if err != nil {
			return false
		}

		block.Params = marshaledIfBlock
		bt[blockName] = block
	}

	return isValid
}

func validateOperand(bt entity.BlocksType, operandInterface interface{}, blockName string) bool {
	if operand, ok := operandInterface.(map[string]interface{}); ok {
		var variableRefInterface interface{}
		if variableRefInterface, ok = operand[variableRefField]; ok {
			var variableRef string
			if variableRef, ok = variableRefInterface.(string); ok {
				schema := &script.JSONSchemaPropertiesValue{
					Type:  stringType,
					Value: variableRef,
				}

				if !isPathValid(bt, schema, blockName) {
					delete(operand, variableRefField)
					return false
				}
			}
		}
	}

	return true
}

func validateNotificationBlock(bt entity.BlocksType, block entity.EriusFunc, blockName string) bool {
	var blockNotification script.NotificationParams
	err := json.Unmarshal(block.Params, &blockNotification)
	if err != nil {
		return false
	}

	isValid := true
	paths := strings.Split(blockNotification.UsersFromSchema, ";")
	var validPaths []string

	for _, path := range paths {
		schema := &script.JSONSchemaPropertiesValue{
			Type:       objectType,
			Properties: people.GetSsoPersonSchemaProperties(),
			Value:      path,
		}

		if !isPathValid(bt, schema, blockName) {
			isValid = false
			continue
		}

		validPaths = append(validPaths, path)
	}

	schema := &script.JSONSchemaPropertiesValue{
		Type:  stringType,
		Value: blockNotification.Text,
	}

	if !isPathValid(bt, schema, blockName) {
		isValid = false
		blockNotification.Text = ""
	}

	if !isValid {
		blockNotification.UsersFromSchema = strings.Join(validPaths, ";")

		var marshaledNotificationBlock []byte
		marshaledNotificationBlock, err = json.Marshal(blockNotification)
		if err != nil {
			return false
		}

		block.Params = marshaledNotificationBlock
		bt[blockName] = block
	}

	return isValid
}

func validateProperties(properties script.JSONSchemaProperties, bt entity.BlocksType, blockName string) bool {
	isValid := true

	for propertyName, property := range properties {
		if property.Value == "" && property.Type == objectType {
			if !validateProperties(property.Properties, bt, blockName) {
				isValid = false
			}
		}

		if !isPathValid(bt, &property, blockName) {
			isValid = false
			property.Value = ""
			properties[propertyName] = property
		}
	}

	return isValid
}

func isPathValid(bt entity.BlocksType, property *script.JSONSchemaPropertiesValue, blockName string) bool {
	if property.Value == "" {
		return true
	}

	path := strings.Split(property.Value, ".")
	targetBlockName := path[0]
	targetBlock, ok := bt[targetBlockName]
	if !ok {
		return false
	}

	if !isBlockBefore(bt, &targetBlock, blockName, map[string]struct{}{}) {
		return false
	}

	path = path[1:]

	variableJSONSchema := searchVariableInJSONSchema(targetBlock.Output.Properties, path)
	if variableJSONSchema == nil {
		return false
	}

	if !isTypeValid(property, variableJSONSchema) {
		return false
	}

	return true
}

func isBlockBefore(
	bt entity.BlocksType,
	targetBlock *entity.EriusFunc,
	currentBlockName string,
	visitedBlocks map[string]struct{}) bool {
	for _, socket := range targetBlock.Next {
		for _, next := range socket {
			if next == currentBlockName {
				return true
			}

			if _, ok := visitedBlocks[next]; ok {
				continue
			}

			visitedBlocks[next] = struct{}{}

			nextBlock, ok := bt[next]
			if !ok {
				return false
			}

			if isBlockBefore(bt, &nextBlock, currentBlockName, visitedBlocks) {
				return true
			}
		}
	}

	return false
}

func searchVariableInJSONSchema(properties script.JSONSchemaProperties, path []string) *script.JSONSchemaPropertiesValue {
	if len(path) == 0 {
		return nil
	}

	param, ok := properties[path[0]]
	if !ok {
		return nil
	}

	if len(path) == 1 {
		return &param
	}

	path = path[1:]

	return searchVariableInJSONSchema(param.Properties, path)
}

func isTypeValid(propertySchema, targetSchema *script.JSONSchemaPropertiesValue) bool {
	if propertySchema == nil || targetSchema == nil {
		return false
	}

	if propertySchema.Type != targetSchema.Type {
		return false
	}

	if propertySchema.Type == objectType {
		if !isObjectValid(propertySchema, targetSchema) {
			return false
		}
	}

	if propertySchema.Type == arrayType {
		if !isArrayValid(propertySchema.Items, targetSchema.Items) {
			return false
		}
	}

	return true
}

func isObjectValid(propertySchema, targetSchema *script.JSONSchemaPropertiesValue) bool {
	for _, requiredProperty := range propertySchema.Required {
		targetProperty, ok := targetSchema.Properties[requiredProperty]
		if !ok {
			return false
		}

		property, ok := propertySchema.Properties[requiredProperty]
		if !ok {
			return false
		}

		if !isTypeValid(&property, &targetProperty) {
			return false
		}
	}

	return true
}

func isArrayValid(propertyItems, targetPropertyItems *script.ArrayItems) bool {
	if propertyItems == nil || targetPropertyItems == nil ||
		propertyItems.Type != targetPropertyItems.Type {
		return false
	}

	if propertyItems.Type == objectType {
		for propertyName, property := range propertyItems.Properties {
			targetProperty, ok := targetPropertyItems.Properties[propertyName]
			if !ok {
				return false
			}

			if !isTypeValid(&property, &targetProperty) {
				return false
			}
		}
	}

	if propertyItems.Type == arrayType {
		if !isArrayValid(propertyItems.Items, targetPropertyItems.Items) {
			return false
		}
	}

	return true
}
