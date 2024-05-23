package db

import (
	"fmt"
	"sort"
	"strings"
)

func mapToString(m map[string]interface{}) string {
	keys := make([]string, 0)

	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	var strBuilder strings.Builder

	for _, k := range keys {
		strBuilder.WriteString(k)
		strBuilder.WriteString(fmt.Sprintf("%v", m[k]))
	}

	return strBuilder.String()
}

func removeDuplicates(slice []interface{}) []interface{} {
	result := make([]interface{}, 0)
	visited := make(map[string]struct{})

	for _, v := range slice {
		mapVal, ok := v.(map[string]interface{})
		if ok {
			mapStr := mapToString(mapVal)
			if _, exists := visited[mapStr]; !exists {
				visited[mapStr] = struct{}{}

				result = append(result, v)
			}
		} else {
			if _, exists := visited[fmt.Sprintf("%v", v)]; !exists {
				visited[fmt.Sprintf("%v", v)] = struct{}{}
				result = append(result, v)
			}
		}
	}

	return result
}
