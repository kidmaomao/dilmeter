package util

func SliceMap[T any, U any](s []T, f func(T) U) []U {
	if len(s) == 0 {
		return nil
	}

	result := make([]U, len(s))
	for i, v := range s {
		result[i] = f(v)
	}

	return result
}

func SliceMap2[T any, U any](s []T, f func(T, int) U) []U {
	if len(s) == 0 {
		return nil
	}

	result := make([]U, len(s))
	for i, v := range s {
		result[i] = f(v, i)
	}

	return result
}
