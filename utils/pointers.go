package utils

func GetAddressOfValue[K any](v K) *K {
	return &v
}
