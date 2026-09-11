package util

func TrySend[T any](ch chan<- T, v T) bool {
	select {
	case ch <- v:
		return true

	default:
		return false
	}
}
