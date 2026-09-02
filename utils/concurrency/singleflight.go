package concurrency

import "golang.org/x/sync/singleflight"

type SingleFlight[T any] struct {
	group singleflight.Group
}

func NewSingleFlight[T any]() *SingleFlight[T] {
	return &SingleFlight[T]{}
}

func (s *SingleFlight[T]) Do(
	key string,
	fn func() (T, error),
) (T, error) {
	value, err, _ := s.group.Do(key, func() (any, error) {
		return fn()
	})

	if err != nil {
		var zero T
		return zero, err
	}

	return value.(T), nil
}
