package db

import "reflect"

func removeDuplicates(slice []interface{}) []interface{} {
	result := make([]interface{}, 0)

	for _, v := range slice {
		duplicate := false

		for _, r := range result {
			if reflect.DeepEqual(v, r) {
				duplicate = true

				break
			}
		}

		if !duplicate {
			result = append(result, v)
		}
	}

	return result
}
