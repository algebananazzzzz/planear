package concurrency

type Task struct {
	ID        int
	Exec      func() error         // Main operation
	OnSuccess func()               // Optional success callback
	OnFailure func(finalErr error) // Optional failure callback
}
