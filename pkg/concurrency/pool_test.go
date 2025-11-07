package concurrency_test

import (
	"errors"
	"sync/atomic"
	"testing"

	"github.com/algebananazzzzz/planear/pkg/concurrency"
)

func flakyFunc(failuresBeforeSuccess int) func() error {
	var attempts int32 = 0
	return func() error {
		current := atomic.AddInt32(&attempts, 1)
		if current <= int32(failuresBeforeSuccess) {
			return errors.New("simulated failure")
		}
		return nil
	}
}

func TestWorkerPool_WithCallbacks(t *testing.T) {
	var successCount int32
	var failureCount int32

	tasks := []concurrency.Task{
		{ // Always succeeds
			ID: 1,
			Exec: func() error {
				return nil
			},
			OnSuccess: func() {
				atomic.AddInt32(&successCount, 1)
			},
			OnFailure: func(err error) {
				t.Logf("Task 1 should not fail: %v", err)
			},
		},
		{ // Succeeds (directly, no retries at pool level)
			ID:   2,
			Exec: func() error { return nil },
			OnSuccess: func() {
				atomic.AddInt32(&successCount, 1)
			},
			OnFailure: func(err error) {
				t.Logf("Task 2 should not fail: %v", err)
			},
		},
		{ // Fails (directly, pool doesn't retry)
			ID:   3,
			Exec: flakyFunc(1), // will fail on first attempt since pool doesn't retry
			OnSuccess: func() {
				t.Errorf("Task 3 should not succeed")
			},
			OnFailure: func(err error) {
				atomic.AddInt32(&failureCount, 1)
			},
		},
	}

	err := concurrency.ExecuteTasks(tasks, 2)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if atomic.LoadInt32(&successCount) != 2 {
		t.Errorf("Expected 2 successes, got %d", successCount)
	}

	if atomic.LoadInt32(&failureCount) != 1 {
		t.Errorf("Expected 1 failure, got %d", failureCount)
	}
}
