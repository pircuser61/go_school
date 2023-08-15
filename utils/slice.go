package utils

import (
	"fmt"
	"strings"

	"golang.org/x/exp/slices"
)

func MergePointersToSlices[T any](args ...*[]T) []T {
	out := make([]T, 0)
	for i := range args {
		if args[i] != nil {
			for j := range *args[i] {
				out = append(out, (*args[i])[j])
			}
		}
	}
	return out
}

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

func FindMin[arrEl any, K []arrEl](arr K, less func(a, b arrEl) bool) (min arrEl, err error) {
	if len(arr) == 0 {
		return min, fmt.Errorf("length of array is %d", len(arr))
	}
	sortedSlice := make(K, len(arr))
	copy(sortedSlice, arr)
	slices.SortFunc(sortedSlice, less)
	return sortedSlice[0], nil
}

func FindMax[arrEl any, K []arrEl](arr K, less func(a, b arrEl) bool) (max arrEl, err error) {
	if len(arr) == 0 {
		return max, fmt.Errorf("length of array is %d", len(arr))
	}

	sortedSlice := make(K, len(arr))
	copy(sortedSlice, arr)
	slices.SortFunc(sortedSlice, less)
	return sortedSlice[len(sortedSlice)-1], nil
}
