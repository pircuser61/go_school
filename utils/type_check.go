package utils

import (
	"fmt"
	"reflect"
)

type TypeValue interface {
	GetType() string
	GetProperties() map[string]interface{}
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
	if value == nil {
		return ""
	}
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
	arrayType:   simpleTypeHandler,

	objectType: nestedTypeHandler,
}

type typeHandler func(variable interface{}, originalValue TypeValue) error

var nestedTypesMapping = map[string]reflect.Kind{
	objectType: reflect.Map,
}

func nestedTypeHandler(variable interface{}, originalValue TypeValue) error {
	nestedType := nestedTypesMapping[originalValue.GetType()]
	variableType := reflect.TypeOf(variable)
	if variableType.Kind() != nestedType {
		return fmt.Errorf("unexpected type of variable %v %T", variable, variable)
	}

	if nestedType == reflect.Map {
		err := handleMap(variable, originalValue)
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
