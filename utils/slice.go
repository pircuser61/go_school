package utils

func UniqueStrings(intSlice []string) []string {
	keys := make(map[string]bool)
	list := make([]string, 0, len(intSlice))
	for _, entry := range intSlice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}

	return list
}
