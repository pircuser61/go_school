package utils

import (
	"fmt"
	"reflect"
)

type TypeValue interface {
	GetType() string
	GetProperties() map[string]interface{}
	GetItems() []interface{}
}

const (
	integerType = "integer"
	stringType  = "string"
	numberType  = "number"
	boolType    = "boolean"
	arrayType   = "array"
	objectType  = "object"
)

func GetJsonType(value interface{}) string {
	switch reflect.TypeOf(value).Kind() {
	case reflect.Int:
		return integerType
	case reflect.Float64:
		return numberType
	case reflect.String:
		return stringType
	case reflect.Bool:
		return boolType
	case reflect.Array:
		return arrayType
	case reflect.Map:
		return objectType
	default:
		return ""
	}
}

func CheckVariableType(variable interface{}, originalValue TypeValue) error {
	tHandler, ok := typesHandlersMapping[originalValue.GetType()]
	if !ok {
		return fmt.Errorf("unexpected type %v", originalValue.GetType())
	}

	return tHandler(variable, originalValue)
}

var typesHandlersMapping = map[string]typeHandler{
	integerType: simpleTypeHandler,
	stringType:  simpleTypeHandler,
	numberType:  simpleTypeHandler,
	boolType:    simpleTypeHandler,

	arrayType:  nestedTypeHandler,
	objectType: nestedTypeHandler,
}

type typeHandler func(variable interface{}, originalValue TypeValue) error

var nestedTypesMapping = map[string]reflect.Kind{
	arrayType:  reflect.Slice,
	objectType: reflect.Map,
}

func nestedTypeHandler(variable interface{}, originalValue TypeValue) error {
	nestedType := nestedTypesMapping[originalValue.GetType()]
	variableType := reflect.TypeOf(variable)
	if variableType.Kind() != nestedType {
		return fmt.Errorf("unexpected type of variable %v %T", variable, variable)
	}

	switch nestedType {
	case reflect.Slice:
		err := handleSlice(variable, originalValue)
		if err != nil {
			return err
		}
	case reflect.Map:
		err := handleMap(variable, originalValue)
		if err != nil {
			return err
		}
	}

	return nil
}

func handleSlice(variable interface{}, originalValue TypeValue) error {
	variableObject := reflect.ValueOf(variable)
	for i, item := range originalValue.GetItems() {
		if i > variableObject.Len() {
			return fmt.Errorf("unexpected length of variable %v", variable)
		}

		err := simpleTypeHandler(variableObject.Index(i).Interface(), item.(TypeValue))
		if err != nil {
			return err
		}
	}

	return nil
}

func handleMap(variable interface{}, originalValue TypeValue) error {
	variableObject := variable.(map[string]interface{})
	for k, v := range originalValue.GetProperties() {
		if _, ok := variableObject[k]; !ok {
			return fmt.Errorf("%v key not found in variable %v", k, variable)
		}

		err := simpleTypeHandler(variableObject[k], v.(TypeValue))
		if err != nil {
			return err
		}
	}

	return nil
}

var simpleTypesMapping = map[string]reflect.Kind{
	integerType: reflect.Int,
	stringType:  reflect.String,
	numberType:  reflect.Float64,
	boolType:    reflect.Bool,
}

func simpleTypeHandler(variable interface{}, originalValue TypeValue) error {
	simpleType, ok := simpleTypesMapping[originalValue.GetType()]
	if !ok {
		return nil
	}

	if reflect.TypeOf(variable).Kind() != simpleType {
		return fmt.Errorf("unexpected type of variable %v %T", variable, variable)
	}

	return nil
}
