package testutils

// NoopValidator returns a validator that always passes.
func NoopValidator[T any]() func(T) error {
	return func(T) error { return nil }
}
