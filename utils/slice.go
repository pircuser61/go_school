package utils

import (
	"strings"

	"golang.org/x/exp/slices"
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

func FindMin[arrEl any, K []arrEl](arr K, less func(a, b arrEl) bool) (min arrEl) {
	sortedSlice := make(K, len(arr))
	copy(sortedSlice, arr)
	slices.SortFunc(sortedSlice, less)
	return sortedSlice[0]
}

func FindMax[arrEl any, K []arrEl](arr K, less func(a, b arrEl) bool) (max arrEl) {
	sortedSlice := make(K, len(arr))
	copy(sortedSlice, arr)
	slices.SortFunc(sortedSlice, less)
	return sortedSlice[len(sortedSlice)-1]
}
