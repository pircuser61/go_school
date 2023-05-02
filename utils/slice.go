package utils

import (
	"strings"
)

func UniqueStrings(inSlice []string) []string {
	keys := make(map[string]bool)
	list := make([]string, 0, len(inSlice))
	for _, entry := range inSlice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}

	return list
}

func IsContainsInSlice(value string, in []string) bool {
	for i := range in {
		if strings.EqualFold(in[i], value) {
			return true
		}
	}

	return false
}
