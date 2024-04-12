package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

const (
	objectOrArrayOrStringType = "objectOrArrayOrString"
	objectType                = "object"
	arrayType                 = "array"
	stringType                = "string"

	conditionGroupsField = "conditionGroups"
	conditionsField      = "conditions"
	leftOperand          = "leftOperand"
	rightOperand         = "rightOperand"
	variableRefField     = "variableRef"
	dataTypeField        = "dataType"

	funcName       = "funcName"
	blockNameLabel = "blockName"
)

func (ae *Env) validatePipeline(ctx context.Context, p *entity.EriusScenario) (valid bool, textErr string) {
	log := logger.GetLogger(ctx)

	ok := validateMappingAndResetIfNotValid(p.Pipeline.Blocks, log)
	if !ok {
		return false, entity.PipelineValidateError
	}

	return p.Pipeline.Blocks.Validate(ctx, ae.ServiceDesc, log)
}

func validateMappingAndResetIfNotValid(bt entity.BlocksType, log logger.Logger) bool {
	isValid := true

	for blockName, block := range bt {
		switch block.TypeID {
		case pipeline.BlockExecutableFunctionID:
			if !validateFunctionBlock(bt, block, blockName, log) {
				isValid = false
			}
		case pipeline.BlockGoApproverID:
			if !validateApproverBlock(bt, block, blockName, log) {
				isValid = false
			}
		case pipeline.BlockGoExecutionID:
			if !validateExecutionBlock(bt, block, blockName, log) {
				isValid = false
			}
		case pipeline.BlockGoFormID:
			if !validateFormBlock(bt, block, blockName, log) {
				isValid = false
			}
		case pipeline.BlockGoSignID:
			if !validateSignBlock(bt, block, blockName, log) {
				isValid = false
			}
		case pipeline.BlockGoIfID:
			if !validateIfBlock(bt, block, blockName, log) {
				isValid = false
			}
		case pipeline.BlockGoNotificationID:
			if !validateNotificationBlock(bt, block, blockName, log) {
				isValid = false
			}
		}
	}

	return isValid
}

func validateFunctionBlock(bt entity.BlocksType, block *entity.EriusFunc, blockName string, log logger.Logger) bool {
	// TODO: use function.Validate
	if block == nil {
		log.WithFields(logger.Fields{
			funcName:       "validateFunctionBlock",
			blockNameLabel: blockName,
		}).Error(errors.New("empty block"))

		return false
	}

	var function script.ExecutableFunctionParams

	err := json.Unmarshal(block.Params, &function)
	if err != nil {
		log.WithFields(logger.Fields{
			funcName:       "validateFunctionBlock",
			blockNameLabel: blockName,
		}).Error(errors.New("failed to unmarshal function params"))

		return false
	}

	newMapping, err := function.GetMappingFromInput()
	if err != nil {
		log.WithFields(logger.Fields{
			funcName:       "getMappingFromInput",
			blockNameLabel: blockName,
		}).Error(errors.New("failed to get new mapping from input params"))

		return false
	}

	function.Mapping = newMapping

	validateRes := validateProperties(function.Mapping, bt, blockName, log)

	if len(function.Constants) != 0 {
		if !validateConstants(function.Constants, function.Mapping, log) {
			validateRes = false
		}
	}

	var marshaledFunction []byte

	marshaledFunction, err = json.Marshal(function)
	if err != nil {
		log.WithFields(logger.Fields{
			funcName:       "validateFunctionBlock",
			blockNameLabel: blockName,
		}).Error(errors.New("failed to marshal function params"))

		return false
	}

	block.Params = marshaledFunction
	bt[blockName] = block

	return validateRes
}

//nolint:dupl //its not duplicate
func validateApproverBlock(bt entity.BlocksType, block *entity.EriusFunc, blockName string, log logger.Logger) bool {
	// TODO: use .Validate()
	if block == nil {
		log.WithFields(logger.Fields{
			funcName:       "validateApproverBlock",
			blockNameLabel: blockName,
		}).Error(errors.New("empty block"))

		return false
	}

	var blockApprover script.ApproverParams

	err := json.Unmarshal(block.Params, &blockApprover)
	if err != nil {
		log.WithFields(logger.Fields{
			funcName:       "validateApproverBlock",
			blockNameLabel: blockName,
		}).Error(errors.New("failed to unmarshal approver params"))

		return false
	}

	isValid := true

	if blockApprover.Type == script.ApproverTypeFromSchema {
		var validVars []string

		approverVars := strings.Split(blockApprover.Approver, ";")
		for _, approverVar := range approverVars {
			schema := &script.JSONSchemaPropertiesValue{
				Type:       objectOrArrayOrStringType,
				Properties: people.GetSsoPersonSchemaProperties(),
				Value:      approverVar,
			}

			if !isPathValid(bt, schema, blockName, log) {
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

		if !isPathValid(bt, schema, blockName, log) {
			isValid = false
			blockApprover.ApproversGroupIDPath = nil
		}
	}

	if !isValid {
		var marshaledApproverBlock []byte

		marshaledApproverBlock, err = json.Marshal(blockApprover)
		if err != nil {
			log.WithFields(logger.Fields{
				funcName:       "validateApproverBlock",
				blockNameLabel: blockName,
			}).Error(errors.New("failed to marshal approver params"))

			return false
		}

		block.Params = marshaledApproverBlock
		bt[blockName] = block
	}

	return isValid
}

//nolint:dupl //its not duplicate
func validateExecutionBlock(bt entity.BlocksType, block *entity.EriusFunc, blockName string, log logger.Logger) bool {
	// TODO: use .Validate()
	if block == nil {
		log.WithFields(logger.Fields{
			funcName:       "validateExecutionBlock",
			blockNameLabel: blockName,
		}).Error(errors.New("empty block"))

		return false
	}

	var blockExecution script.ExecutionParams

	err := json.Unmarshal(block.Params, &blockExecution)
	if err != nil {
		log.WithFields(logger.Fields{
			funcName:       "validateExecutionBlock",
			blockNameLabel: blockName,
		}).Error(errors.New("failed to unmarshal execution params"))

		return false
	}

	isValid := true

	if blockExecution.Type == script.ExecutionTypeFromSchema {
		var validVars []string

		executorVars := strings.Split(blockExecution.Executors, ";")
		for _, executorVar := range executorVars {
			schema := &script.JSONSchemaPropertiesValue{
				Type:       objectOrArrayOrStringType,
				Properties: people.GetSsoPersonSchemaProperties(),
				Value:      executorVar,
			}

			if !isPathValid(bt, schema, blockName, log) {
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

		if !isPathValid(bt, schema, blockName, log) {
			isValid = false
			blockExecution.ExecutorsGroupIDPath = nil
		}
	}

	if !isValid {
		var marshaledExecutionBlock []byte

		marshaledExecutionBlock, err = json.Marshal(blockExecution)
		if err != nil {
			log.WithFields(logger.Fields{
				funcName:       "validateExecutionBlock",
				blockNameLabel: blockName,
			}).Error(errors.New("failed to marshal execution params"))

			return false
		}

		block.Params = marshaledExecutionBlock
		bt[blockName] = block
	}

	return isValid
}

func validateFormBlock(bt entity.BlocksType, block *entity.EriusFunc, blockName string, log logger.Logger) bool {
	// TODO: use .Validate()
	if block == nil {
		log.WithFields(logger.Fields{
			funcName:       "validateFormBlock",
			blockNameLabel: blockName,
		}).Error(errors.New("empty block"))

		return false
	}

	var blockForm script.FormParams

	err := json.Unmarshal(block.Params, &blockForm)
	if err != nil {
		log.WithFields(logger.Fields{
			funcName:       "validateFormBlock",
			blockNameLabel: blockName,
		}).Error(errors.New("failed to unmarshal form params"))

		return false
	}

	isValid := true
	if !validateProperties(blockForm.Mapping, bt, blockName, log) {
		isValid = false
	}

	objectSchema := &script.JSONSchemaPropertiesValue{
		Type:  objectType,
		Value: blockForm.FullFormMapping,
	}

	if !isPathValid(bt, objectSchema, blockName, log) {
		isValid = false
		blockForm.FullFormMapping = ""
	}

	if blockForm.FormExecutorType == script.FormExecutorTypeFromSchema {
		var validVars []string

		executorVars := strings.Split(blockForm.Executor, ";")
		for _, executorVar := range executorVars {
			schema := &script.JSONSchemaPropertiesValue{
				Type:       objectOrArrayOrStringType,
				Properties: people.GetSsoPersonSchemaProperties(),
				Value:      executorVar,
			}

			if !isPathValid(bt, schema, blockName, log) {
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

		if !isPathValid(bt, schema, blockName, log) {
			isValid = false
			blockForm.FormGroupIDPath = nil
		}
	}

	if len(blockForm.Constants) != 0 {
		if !validateConstants(blockForm.Constants, blockForm.Mapping, log) {
			isValid = false
		}
	}

	if !isValid {
		var marshaledForm []byte

		marshaledForm, err = json.Marshal(blockForm)
		if err != nil {
			log.WithFields(logger.Fields{
				funcName:       "validateFormBlock",
				blockNameLabel: blockName,
			}).Error(errors.New("failed to marshal form params"))

			return false
		}

		block.Params = marshaledForm
		bt[blockName] = block
	}

	return isValid
}

func validateSignBlock(bt entity.BlocksType, block *entity.EriusFunc, blockName string, log logger.Logger) bool {
	// TODO: use .Validate()
	if block == nil {
		log.WithFields(logger.Fields{
			funcName:       "validateSignBlock",
			blockNameLabel: blockName,
		}).Error(errors.New("empty block"))

		return false
	}

	var blockSign script.SignParams

	err := json.Unmarshal(block.Params, &blockSign)
	if err != nil {
		log.WithFields(logger.Fields{
			funcName:       "validateSignBlock",
			blockNameLabel: blockName,
		}).Error(errors.New("failed to unmarshal sign params"))

		return false
	}

	isValid := true

	if blockSign.Type == script.SignerTypeFromSchema {
		var validVars []string

		signerVars := strings.Split(blockSign.Signer, ";")
		for _, signerVar := range signerVars {
			schema := &script.JSONSchemaPropertiesValue{
				Type:       objectOrArrayOrStringType,
				Properties: people.GetSsoPersonSchemaProperties(),
				Value:      signerVar,
			}

			if !isPathValid(bt, schema, blockName, log) {
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

		if !isPathValid(bt, schema, blockName, log) {
			isValid = false
			blockSign.SignerGroupIDPath = ""
		}
	}

	schema := &script.JSONSchemaPropertiesValue{
		Type:  stringType,
		Value: blockSign.SigningParamsPaths.SNILS,
	}

	if !isPathValid(bt, schema, blockName, log) {
		isValid = false
		blockSign.SigningParamsPaths.SNILS = ""
	}

	schema.Value = blockSign.SigningParamsPaths.INN

	if !isPathValid(bt, schema, blockName, log) {
		isValid = false
		blockSign.SigningParamsPaths.INN = ""
	}

	validFiles := make([]string, 0, len(blockSign.SigningParamsPaths.Files))

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

		if !isPathValid(bt, fileSchema, blockName, log) {
			isValid = false

			continue
		}

		validFiles = append(validFiles, fileRef)
	}

	if len(validFiles) != len(blockSign.SigningParamsPaths.Files) {
		blockSign.SigningParamsPaths.Files = validFiles
	}

	if !isValid {
		var marshaledSignBlock []byte

		marshaledSignBlock, err = json.Marshal(blockSign)
		if err != nil {
			log.WithFields(logger.Fields{
				funcName:       "validateSignBlock",
				blockNameLabel: blockName,
			}).Error(errors.New("failed to marshal sign params"))

			return false
		}

		block.Params = marshaledSignBlock
		bt[blockName] = block
	}

	return isValid
}

func validateIfBlock(bt entity.BlocksType, block *entity.EriusFunc, blockName string, log logger.Logger) bool {
	if block == nil {
		log.WithFields(logger.Fields{
			funcName:       "validateIfBlock",
			blockNameLabel: blockName,
		}).Error(errors.New("empty block"))

		return false
	}

	var blockIf map[string]interface{}

	err := json.Unmarshal(block.Params, &blockIf)
	if err != nil {
		log.WithFields(logger.Fields{
			funcName:       "validateIfBlock",
			blockNameLabel: blockName,
		}).Error(errors.New("failed to unmarshal block params"))

		return false
	}

	conditionGroupsInterface, ok := blockIf[conditionGroupsField]
	if !ok {
		log.WithField(funcName, "validateIfBlock").Error(errors.New("condition groups are missing"))

		return false
	}

	conditionGroups, ok := conditionGroupsInterface.([]interface{})
	if !ok {
		log.WithField(funcName, "validateIfBlock").
			Error(errors.New("failed to cast condition groups to []interface{}"))

		return false
	}

	isValid := true

	for _, groupInterface := range conditionGroups {
		var group map[string]interface{}

		group, ok = groupInterface.(map[string]interface{})
		if !ok {
			log.WithField(funcName, "validateIfBlock").
				Error(errors.New("failed to cast condition group to map[string]interface{}"))

			isValid = false

			continue
		}

		if !validateConditionGroup(bt, group, blockName, log) {
			isValid = false
		}
	}

	if !isValid {
		var marshaledIfBlock []byte

		marshaledIfBlock, err = json.Marshal(blockIf)
		if err != nil {
			log.WithFields(logger.Fields{
				funcName:       "validateIfBlock",
				blockNameLabel: blockName,
			}).Error(errors.New("failed to marshal if block params"))

			return false
		}

		block.Params = marshaledIfBlock
		bt[blockName] = block
	}

	return isValid
}

func validateConditionGroup(bt entity.BlocksType, group map[string]interface{}, blockName string, log logger.Logger) bool {
	conditionsInterface, ok := group[conditionsField]
	if !ok {
		log.WithFields(logger.Fields{
			funcName:       "validateConditionGroup",
			blockNameLabel: blockName,
		}).Error(errors.New("conditions not found"))

		return false
	}

	var conditions []interface{}

	conditions, ok = conditionsInterface.([]interface{})
	if !ok {
		log.WithField(funcName, "validateConditionGroup").
			Error(errors.New("failed to cast conditions to []interface{}"))

		return false
	}

	isValid := true

	for _, conditionInterface := range conditions {
		var condition map[string]interface{}

		condition, ok = conditionInterface.(map[string]interface{})
		if !ok {
			log.WithField(funcName, "validateConditionGroup").
				Error(errors.New("failed to cast condition to map[string]interface{}"))

			isValid = false

			continue
		}

		var operandInterface interface{}
		if operandInterface, ok = condition[leftOperand]; ok {
			if !validateOperand(bt, operandInterface, blockName, log) {
				isValid = false
			}
		}

		if operandInterface, ok = condition[rightOperand]; ok {
			if !validateOperand(bt, operandInterface, blockName, log) {
				isValid = false
			}
		}
	}

	return isValid
}

func validateOperand(bt entity.BlocksType, operandInterface interface{}, blockName string, log logger.Logger) bool {
	operand, ok := operandInterface.(map[string]interface{})
	if !ok {
		log.WithField(funcName, "validateOperand").
			Error(errors.New("failed to cast operand to map[string]interface{}"))

		return false
	}

	var variableRefInterface interface{}

	variableRefInterface, ok = operand[variableRefField]
	if !ok {
		return true
	}

	var variableRef string

	variableRef, ok = variableRefInterface.(string)
	if !ok {
		log.WithField(funcName, "validateOperand").
			Error(errors.New("failed to cast variableRef to string"))

		return false
	}

	dataTypeInterface, ok := operand[dataTypeField]
	if !ok {
		log.WithField(funcName, "validateOperand").
			Error(errors.New("dataType is empty"))

		return false
	}

	var typeField string

	typeField, ok = dataTypeInterface.(string)
	if !ok {
		log.WithField(funcName, "validateOperand").
			Error(errors.New("failed to cast dataType to string"))

		return false
	}

	schema := &script.JSONSchemaPropertiesValue{
		Type:  typeField,
		Value: variableRef,
	}

	if !isPathValid(bt, schema, blockName, log) {
		delete(operand, variableRefField)

		return false
	}

	return true
}

func validateNotificationBlock(bt entity.BlocksType, block *entity.EriusFunc, blockName string, log logger.Logger) bool {
	// TODO: use .Validate()
	if block == nil {
		log.WithFields(logger.Fields{
			funcName:       "validateNotificationBlock",
			blockNameLabel: blockName,
		}).Error(errors.New("empty block"))

		return false
	}

	var blockNotification script.NotificationParams

	err := json.Unmarshal(block.Params, &blockNotification)
	if err != nil {
		log.WithFields(logger.Fields{
			funcName:       "validateNotificationBlock",
			blockNameLabel: blockName,
		}).Error(errors.New("failed to unmarshal notification params"))

		return false
	}

	isValid := true
	paths := strings.Split(blockNotification.UsersFromSchema, ";")
	validPaths := make([]string, 0, len(paths))

	for _, path := range paths {
		schema := &script.JSONSchemaPropertiesValue{
			Type:       objectOrArrayOrStringType,
			Properties: people.GetSsoPersonSchemaProperties(),
			Value:      path,
		}

		if !isPathValid(bt, schema, blockName, log) {
			isValid = false

			continue
		}

		validPaths = append(validPaths, path)
	}

	if blockNotification.TextSourceType == "context" {
		schema := &script.JSONSchemaPropertiesValue{
			Type:  stringType,
			Value: blockNotification.Text,
		}

		if !isPathValid(bt, schema, blockName, log) {
			isValid = false
			blockNotification.Text = ""
		}
	}

	if !isValid {
		blockNotification.UsersFromSchema = strings.Join(validPaths, ";")

		var marshaledNotificationBlock []byte

		marshaledNotificationBlock, err = json.Marshal(blockNotification)
		if err != nil {
			log.WithFields(logger.Fields{
				funcName:       "validateNotificationBlock",
				blockNameLabel: blockName,
			}).Error(errors.New("failed to marshal notification params"))

			return false
		}

		block.Params = marshaledNotificationBlock
		bt[blockName] = block
	}

	return isValid
}

// nolint:gocritic,gosec // don't want pointer of JSONSchemaPropertiesValue
func validateProperties(properties script.JSONSchemaProperties, bt entity.BlocksType, blockName string, log logger.Logger) bool {
	isValid := true

	for propertyName, property := range properties {
		if property.Value == "" && property.Type == objectType {
			if !validateProperties(property.Properties, bt, blockName, log) {
				isValid = false
			}
		}

		if !isPathValid(bt, &property, blockName, log) {
			isValid = false
			property.Value = ""
			properties[propertyName] = property
		}
	}

	return isValid
}

func isPathValid(bt entity.BlocksType, property *script.JSONSchemaPropertiesValue, blockName string, log logger.Logger) bool {
	if property.Value == "" {
		return true
	}

	path := strings.Split(property.Value, ".")
	targetBlockName := path[0]

	targetBlock, ok := bt[targetBlockName]
	if !ok {
		log.WithFields(logger.Fields{
			funcName:       "isPathValid",
			blockNameLabel: blockName,
		}).Error(fmt.Errorf("mapping is not valid, block %s is missing, %s", targetBlockName, property.Value))

		return false
	}

	if !isBlockBefore(bt, targetBlock, blockName, map[string]struct{}{}) {
		log.WithFields(logger.Fields{
			funcName:       "isPathValid",
			blockNameLabel: blockName,
		}).Error(fmt.Errorf("mapping is not valid, block %s is not in context, %s", targetBlockName, property.Value))

		return false
	}

	path = path[1:]

	variableJSONSchema := searchVariableInJSONSchema(targetBlock.Output.Properties, path)
	if variableJSONSchema == nil {
		log.WithFields(logger.Fields{
			funcName:       "isPathValid",
			blockNameLabel: blockName,
		}).Error(fmt.Errorf("mapping is not valid, variable %s is missing in block %s", property.Value, targetBlockName))

		return false
	}

	if !isTypeValid(property, variableJSONSchema) {
		log.WithFields(logger.Fields{
			funcName:       "isPathValid",
			blockNameLabel: blockName,
		}).Error(fmt.Errorf("mapping is not valid, variable is not the same type, %s", property.Value))

		return false
	}

	return true
}

func isBlockBefore(
	bt entity.BlocksType,
	targetBlock *entity.EriusFunc,
	currentBlockName string,
	visitedBlocks map[string]struct{},
) bool {
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

			if isBlockBefore(bt, nextBlock, currentBlockName, visitedBlocks) {
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

	if propertySchema.Type != targetSchema.Type &&
		!(propertySchema.Type == objectOrArrayOrStringType &&
			(targetSchema.Type == objectType || targetSchema.Type == arrayType || targetSchema.Type == stringType)) {
		return false
	}

	if propertySchema.Type == objectType || (propertySchema.Type == objectOrArrayOrStringType && targetSchema.Type == objectType) {
		if !isObjectValid(propertySchema, targetSchema) {
			return false
		}
	}

	if propertySchema.Type == arrayType {
		if !isArrayValid(propertySchema.Items, targetSchema.Items) {
			return false
		}
	}

	if propertySchema.Type == objectOrArrayOrStringType && targetSchema.Type == arrayType {
		items := &script.ArrayItems{
			Type:       objectType,
			Format:     "ssoperson",
			Properties: people.GetSsoPersonSchemaProperties(),
		}
		if !isArrayValid(items, targetSchema.Items) {
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

// nolint:gocritic,gosec // don't want pointer of JSONSchemaPropertiesValue
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

func validateConstants(constants map[string]interface{}, mapping script.JSONSchemaProperties, log logger.Logger) bool {
	isValid := true

	for constPath, value := range constants {
		pathParts := strings.Split(constPath, ".")

		if !validateConstant(pathParts, value, mapping, log) {
			isValid = false

			delete(constants, constPath)
		}
	}

	return isValid
}

func validateConstant(pathParts []string, value interface{}, schema script.JSONSchemaProperties, log logger.Logger) bool {
	part := pathParts[0]
	if part == "" {
		log.WithField(funcName, "validateConstant").Error(fmt.Errorf("constant %v is not valid, empty path", value))

		return false
	}

	paramJSONSchema, ok := schema[part]
	if !ok {
		log.WithField(funcName, "validateConstant").
			Error(fmt.Errorf("constant %s %v is not valid, not found in schema", strings.Join(pathParts, "."), value))

		return false
	}

	if len(pathParts) == 1 {
		err := script.ValidateParam(value, &paramJSONSchema)
		if err != nil {
			log.WithField(funcName, "validateConstant").WithError(err).
				Error(fmt.Errorf("constant %s %v is not valid, does not match schema", strings.Join(pathParts, "."), value))

			return false
		}

		return true
	}

	return validateConstant(pathParts[1:], value, paramJSONSchema.Properties, log)
}
