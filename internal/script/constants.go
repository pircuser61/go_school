package script

import (
	"errors"
	"strings"
)

func FillMapWithConstants(consts, functionMapping map[string]interface{}) error {
	for keyName, value := range consts {
		keyParts := strings.Split(keyName, ".")
		currMap := functionMapping

		for i, part := range keyParts {
			if i == len(keyParts)-1 {
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
