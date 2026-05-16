package apply

import (
	"fmt"
	"runtime"
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
	FinalizeOn      types.FinalizeOn
}

func ExecuteOperations[T any](params ExecuteOperationsParams[T]) (*types.ExecutionReport[T], error) {
	// Validate required function parameters
	if params.FormatRecord == nil {
		return nil, fmt.Errorf("FormatRecord is required")
	}
	if params.FormatKey == nil {
		return nil, fmt.Errorf("FormatKey is required")
	}
	if params.OnAdd == nil {
		return nil, fmt.Errorf("OnAdd is required")
	}
	if params.OnUpdate == nil {
		return nil, fmt.Errorf("OnUpdate is required")
	}
	if params.OnDelete == nil {
		return nil, fmt.Errorf("OnDelete is required")
	}

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

	if params.Parallelization == nil {
		defaultParallelism := runtime.NumCPU()
		params.Parallelization = &defaultParallelism
	}

	var skipped types.Plan[T]

	if params.Plan.Layers != nil {
		// --- Layered path ---
		if err := verifyLayersMultiset(params.Plan); err != nil {
			return nil, err
		}

		addByKey := make(map[string]types.RecordAddition[T], len(params.Plan.Additions))
		for _, a := range params.Plan.Additions {
			addByKey[a.Key] = a
		}
		updByKey := make(map[string]types.RecordUpdate[T], len(params.Plan.Updates))
		for _, u := range params.Plan.Updates {
			updByKey[u.Key] = u
		}
		delByKey := make(map[string]types.RecordDeletion[T], len(params.Plan.Deletions))
		for _, d := range params.Plan.Deletions {
			delByKey[d.Key] = d
		}

		stopAfter := -1
		for layerIdx, layer := range params.Plan.Layers {
			layerTasks := make([]concurrency.Task, 0, len(layer))
			for _, op := range layer {
				switch op.Kind {
				case types.LayerOpAdd:
					layerTasks = append(layerTasks, addTask(addByKey[op.Key]))
				case types.LayerOpUpdate:
					layerTasks = append(layerTasks, updateTask(updByKey[op.Key]))
				case types.LayerOpDelete:
					layerTasks = append(layerTasks, deleteTask(delByKey[op.Key]))
				default:
					return nil, fmt.Errorf("plan.Layers unknown op kind %q at layer %d", op.Kind, layerIdx)
				}
			}

			failBefore := len(failure.Additions) + len(failure.Updates) + len(failure.Deletions)
			if err := concurrency.ExecuteTasks(layerTasks, *params.Parallelization); err != nil {
				return nil, fmt.Errorf("failed to execute layer %d: %w", layerIdx, err)
			}
			failAfter := len(failure.Additions) + len(failure.Updates) + len(failure.Deletions)
			if failAfter > failBefore {
				stopAfter = layerIdx
				break
			}
		}

		if stopAfter >= 0 {
			for _, layer := range params.Plan.Layers[stopAfter+1:] {
				for _, op := range layer {
					switch op.Kind {
					case types.LayerOpAdd:
						skipped.Additions = append(skipped.Additions, addByKey[op.Key])
					case types.LayerOpUpdate:
						skipped.Updates = append(skipped.Updates, updByKey[op.Key])
					case types.LayerOpDelete:
						skipped.Deletions = append(skipped.Deletions, delByKey[op.Key])
					}
				}
			}
		}
	} else {
		// --- Flat path (existing behavior) ---
		// Execute deletions first to free up resources/identifiers before additions/updates
		for _, del := range params.Plan.Deletions {
			tasks = append(tasks, deleteTask(del))
		}
		for _, rec := range params.Plan.Additions {
			tasks = append(tasks, addTask(rec))
		}
		for _, upd := range params.Plan.Updates {
			tasks = append(tasks, updateTask(upd))
		}

		if err := concurrency.ExecuteTasks(tasks, *params.Parallelization); err != nil {
			return nil, fmt.Errorf("failed to complete all operations: %w", err)
		}
	}

	// Create report (will be printed by caller before finalization errors are reported)
	report := &types.ExecutionReport[T]{
		Success:             success,
		Failure:             failure,
		Skipped:             skipped,
		Ignores:             params.Plan.Ignores,
		FinalizationSuccess: true, // Default to true, set to false if finalization fails
	}

	// Execute finalization with retry and exponential backoff (errors will be reported by caller after execution report)
	var finalizeErr error
	if params.OnFinalize != nil && shouldRunFinalize(params.FinalizeOn, report) {
		// Retry finalization with the same retry and logging pattern
		if err := retryWithLogging(params.OnFinalize, "finalize", ""); err != nil {
			report.FinalizationSuccess = false
			report.FinalizationErrorMsg = err.Error()
			finalizeErr = err
		}
	}

	return report, finalizeErr
}

// shouldRunFinalize returns true when the configured FinalizeOn policy permits
// invoking OnFinalize given the current execution report.
func shouldRunFinalize[T any](policy types.FinalizeOn, report *types.ExecutionReport[T]) bool {
	switch policy {
	case types.FinalizeOnSuccess:
		anyFail := !report.Failure.IsEmpty() || !report.Skipped.IsEmpty()
		return !anyFail
	case types.FinalizeOnAnySuccess:
		return !report.Success.IsEmpty()
	case types.FinalizeAlways:
		fallthrough
	default:
		return true
	}
}

// verifyLayersMultiset ensures every op in plan.Additions/Updates/Deletions
// appears exactly once in plan.Layers, and that plan.Layers does not reference
// any op not present in the plan. Mismatches (likely from a stale or hand-edited
// plan file) are surfaced as errors before any DB write occurs.
func verifyLayersMultiset[T any](plan types.Plan[T]) error {
	want := map[types.LayerOp]int{}
	for _, a := range plan.Additions {
		want[types.LayerOp{Kind: types.LayerOpAdd, Key: a.Key}]++
	}
	for _, u := range plan.Updates {
		want[types.LayerOp{Kind: types.LayerOpUpdate, Key: u.Key}]++
	}
	for _, d := range plan.Deletions {
		want[types.LayerOp{Kind: types.LayerOpDelete, Key: d.Key}]++
	}
	got := map[types.LayerOp]int{}
	for _, layer := range plan.Layers {
		for _, op := range layer {
			got[op]++
		}
	}
	for k, v := range want {
		if got[k] != v {
			return fmt.Errorf("plan.Layers multiset mismatch: op %+v expected %d times, found %d (plan may be stale or hand-edited)", k, v, got[k])
		}
	}
	for k, v := range got {
		if want[k] != v {
			return fmt.Errorf("plan.Layers references unknown op %+v %d time(s)", k, v)
		}
	}
	return nil
}
