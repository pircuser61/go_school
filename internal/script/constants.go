package script

import (
	"errors"
	"strings"
)

func FillFuncMapWithConstants(constants, mapData map[string]interface{}) error {
	for constantName, value := range constants {
		constantNameParts := strings.Split(constantName, ".")
		currMap := mapData

		for i, part := range constantNameParts {
			if i == len(constantNameParts)-1 {
				currMap[part] = value

				break
			}

			newCurrMap, ok := currMap[part]
			if !ok {
				newCurrMap = make(map[string]interface{})
				currMap[part] = newCurrMap
			}

			convNewCurrMap, ok := newCurrMap.(map[string]interface{})
			if !ok {
				return errors.New("can`t assert newCurrMap to map[string]interface{}")
			}

			currMap = convNewCurrMap
		}
	}

	return nil
}

func FillFormMapWithConstants(constants, mapData map[string]interface{}) error {
	for constantName, value := range constants {
		constantNameParts := strings.Split(constantName, ".")
		currMap := mapData

		for i, part := range constantNameParts {
			if i == len(constantNameParts)-1 {
				currMap[part] = value

				break
			}
		}
	}

	return nil
}
