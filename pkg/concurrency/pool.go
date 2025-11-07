package concurrency

import (
	"fmt"
	"sync"
	"time"
)

// RetryWithBackoff retries a function up to 3 times with exponential backoff
func RetryWithBackoff(op func() error, maxRetries int, baseDelay time.Duration) error {
	var err error
	for i := 0; i <= maxRetries; i++ {
		err = op()
		if err == nil {
			return nil
		}
		time.Sleep(baseDelay * (1 << i)) // exponential backoff
	}
	return fmt.Errorf("operation failed after %d retries: (%w)", maxRetries, err)
}

func ExecuteTasks(tasks []Task, workerCount int) error {
	taskCh := make(chan Task)
	errCh := make(chan error, len(tasks))
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		for task := range taskCh {
			// Execute the task directly (retries are handled by task.Exec via retryWithLogging)
			err := task.Exec()
			if err != nil {
				if task.OnFailure != nil {
					task.OnFailure(err)
				}
				errCh <- fmt.Errorf("task %d failed: %w", task.ID, err)
			} else {
				if task.OnSuccess != nil {
					task.OnSuccess()
				}
			}
		}
	}

	// Start workers
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go worker()
	}

	// Send tasks
	go func() {
		defer close(taskCh)
		for _, task := range tasks {
			taskCh <- task
		}
	}()

	wg.Wait()
	close(errCh)

	return nil
}
