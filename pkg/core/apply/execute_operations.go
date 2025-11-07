package apply

import (
	"fmt"
	"time"

	"github.com/algebananazzzzz/planear/pkg/concurrency"
	"github.com/algebananazzzzz/planear/pkg/constants"
	"github.com/algebananazzzzz/planear/pkg/formatters"
	"github.com/algebananazzzzz/planear/pkg/types"
)

type ExecuteOperationsParams[T any] struct {
	Plan            types.Plan[T]
	FormatRecord    func(T) string
	FormatKey       func(string) string
	OnAdd           func(types.RecordAddition[T]) error
	OnUpdate        func(types.RecordUpdate[T]) error
	OnDelete        func(types.RecordDeletion[T]) error
	OnFinalize      func() error
	Parallelization *int
}

func ExecuteOperations[T any](params ExecuteOperationsParams[T]) (*types.ExecutionReport[T], error) {
	var success types.Plan[T]
	var failure types.Plan[T]
	var tasks []concurrency.Task

	// Helper function to retry operations with exponential backoff and logging
	retryWithLogging := func(op func() error, opType string, formatted string) error {
		const maxRetries = 3
		const baseDelay = 100 * time.Millisecond

		for attempt := 1; attempt <= maxRetries; attempt++ {
			err := op()
			if err == nil {
				return nil
			}

			// Log the failure in red with attempt count
			var logMsg string
			if formatted == "" {
				// For finalization or operations without a record
				logMsg = fmt.Sprintf("%sRETRY: %s failed (attempt %d) - %v%s",
					constants.ColorRed, opType, attempt, err, constants.ColorReset)
			} else {
				logMsg = fmt.Sprintf("%sRETRY: Failed to %s %s (attempt %d) - %v%s",
					constants.ColorRed, opType, formatted, attempt, err, constants.ColorReset)
			}
			fmt.Println(logMsg)

			// If not the last attempt, wait with exponential backoff
			if attempt < maxRetries {
				time.Sleep(baseDelay * time.Duration(1<<(attempt-1)))
			}
		}

		return fmt.Errorf("operation failed after %d retries", maxRetries)
	}

	addTask := func(rec types.RecordAddition[T]) concurrency.Task {
		return concurrency.Task{
			Exec: func() error {
				return retryWithLogging(
					func() error { return params.OnAdd(rec) },
					"add",
					params.FormatRecord(rec.New),
				)
			},
			OnSuccess: func() {
				success.Additions = append(success.Additions, rec)
			},
			OnFailure: func(err error) {
				failure.Additions = append(failure.Additions, rec)
				fmt.Printf("%s[ADD FAILED] Unable to add record: %v\nReason: %s%s\n",
					constants.ColorRed, params.FormatRecord(rec.New),
					err, constants.ColorReset)
			},
		}
	}

	updateTask := func(upd types.RecordUpdate[T]) concurrency.Task {
		return concurrency.Task{
			Exec: func() error {
				return retryWithLogging(
					func() error { return params.OnUpdate(upd) },
					"update",
					params.FormatRecord(upd.New),
				)
			},
			OnSuccess: func() {
				success.Updates = append(success.Updates, upd)
			},
			OnFailure: func(err error) {
				failure.Updates = append(failure.Updates, upd)
				fmt.Printf("%s[UPDATE FAILED] Unable to update record: %v\nReason: %s%s\n",
					constants.ColorRed, formatters.FormatUpdate(upd, params.FormatKey),
					err, constants.ColorReset)
			},
		}
	}

	deleteTask := func(del types.RecordDeletion[T]) concurrency.Task {
		return concurrency.Task{
			Exec: func() error {
				return retryWithLogging(
					func() error { return params.OnDelete(del) },
					"delete",
					params.FormatRecord(del.Old),
				)
			},
			OnSuccess: func() {
				success.Deletions = append(success.Deletions, del)
			},
			OnFailure: func(err error) {
				failure.Deletions = append(failure.Deletions, del)
				fmt.Printf("%s[Delete Failed] Unable to delete record: %v\nReason: %s%s\n",
					constants.ColorRed, params.FormatRecord(del.Old),
					err, constants.ColorReset)
			},
		}
	}

	for _, rec := range params.Plan.Additions {
		tasks = append(tasks, addTask(rec))
	}
	for _, upd := range params.Plan.Updates {
		tasks = append(tasks, updateTask(upd))
	}
	for _, del := range params.Plan.Deletions {
		tasks = append(tasks, deleteTask(del))
	}

	if params.Parallelization == nil {
		defaultParallelism := 2
		params.Parallelization = &defaultParallelism
	}

	if err := concurrency.ExecuteTasks(tasks, *params.Parallelization); err != nil {
		return nil, fmt.Errorf("failed to complete all operations: %w", err)
	}

	// Create report (will be printed by caller before finalization errors are reported)
	report := &types.ExecutionReport[T]{
		Success:             success,
		Failure:             failure,
		Ignores:             params.Plan.Ignores,
		FinalizationSuccess: true, // Default to true, set to false if finalization fails
	}

	// Execute finalization with retry and exponential backoff (errors will be reported by caller after execution report)
	var finalizeErr error
	if params.OnFinalize != nil {
		// Retry finalization with the same retry and logging pattern
		if err := retryWithLogging(params.OnFinalize, "finalize", ""); err != nil {
			report.FinalizationSuccess = false
			report.FinalizationErrorMsg = err.Error()
			finalizeErr = err
		}
	}

	return report, finalizeErr
}
