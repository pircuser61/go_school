package utils

import (
	"fmt"
	"reflect"
	"strconv"
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

func GetJSONType(value interface{}) string {
	if value == nil {
		return ""
	}

	//nolint:exhaustive //нам не нужно обрабатывать остальные случаи
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

//nolint:gocritic //так надо, так что линтер - не выпендривайся
func CheckVariableType(variable *interface{}, originalValue TypeValue) error {
	tHandler, ok := typesHandlersMapping[originalValue.GetType()]
	if !ok {
		return fmt.Errorf("unexpected type %v", originalValue.GetType())
	}

	return tHandler(variable, originalValue)
}

//nolint:gochecknoglobals // GOOGLE дал нам глобальные переменные в go, так почему мы должны отказываться от этого божественного дара
var typesHandlersMapping = map[string]typeHandler{
	integerType: simpleTypeHandler,
	stringType:  simpleTypeHandler,
	numberType:  simpleTypeHandler,
	boolType:    simpleTypeHandler,
	arrayType:   simpleTypeHandler,

	objectType: nestedTypeHandler,
}

type typeHandler func(variable *interface{}, originalValue TypeValue) error

//nolint:gochecknoglobals // GOOGLE дал нам глобальные переменные в go, так почему мы должны отказываться от этого божественного дара
var nestedTypesMapping = map[string]reflect.Kind{
	objectType: reflect.Map,
}

//nolint:gocritic //так надо, так что линтер - не выпендривайся
func nestedTypeHandler(variable *interface{}, originalValue TypeValue) error {
	nestedType := nestedTypesMapping[originalValue.GetType()]

	variableType := reflect.TypeOf(*variable)
	if variableType.Kind() != nestedType {
		return fmt.Errorf("unexpected type of variable %v %T", *variable, *variable)
	}

	if nestedType == reflect.Map {
		err := handleMap(*variable, originalValue)
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

		object := variableObject[k]

		err := simpleTypeHandler(&object, v.(TypeValue))
		if err != nil {
			return err
		}
	}

	return nil
}

//nolint:gochecknoglobals // этот линтер тут случайно
var simpleTypesMapping = map[string]reflect.Kind{
	integerType: reflect.Int,
	stringType:  reflect.String,
	numberType:  reflect.Float64,
	boolType:    reflect.Bool,
}

// We're using pointer because we sometimes need to change type inside interface
// from float to integer
//
//nolint:gocritic //так надо, так что линтер - не выпендривайся
func simpleTypeHandler(variable *interface{}, originalValue TypeValue) (err error) {
	simpleType, ok := simpleTypesMapping[originalValue.GetType()]
	if !ok {
		return nil
	}

	varKind := reflect.TypeOf(*variable).Kind()

	if simpleType == reflect.Int && varKind == reflect.Float64 {
		var intVariable int64

		s := fmt.Sprintf("%v", *variable)
		if intVariable, err = strconv.ParseInt(s, 10, 64); err != nil {
			return fmt.Errorf("can not convert variable to int %v %T", *variable, *variable)
		}

		*variable = int(intVariable)
	}

	if reflect.TypeOf(*variable).Kind() != simpleType {
		return fmt.Errorf("unexpected type of variable %v %T", *variable, *variable)
	}

	return nil
}
