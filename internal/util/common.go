package util

func MargeMaps[K comparable, V any](into map[K]V, from map[K]V) map[K]V {
	if into == nil {
		into = make(map[K]V, len(from))
	}

	for key, value := range from {
		into[key] = value
	}

	return into
}
