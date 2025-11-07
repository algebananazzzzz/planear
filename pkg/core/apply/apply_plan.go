package apply

import (
	"fmt"

	"github.com/algebananazzzzz/planear/pkg/constants"
	"github.com/algebananazzzzz/planear/pkg/formatters"
	"github.com/algebananazzzzz/planear/pkg/types"
	"github.com/algebananazzzzz/planear/pkg/utils"
)

type RunParams[T any] struct {
	PlanFilePath    string
	FormatRecord    func(T) string
	FormatKey       func(string) string
	OnAdd           func(types.RecordAddition[T]) error
	OnUpdate        func(types.RecordUpdate[T]) error
	OnDelete        func(types.RecordDeletion[T]) error
	OnFinalize      func() error
	Parallelization *int
}

func Run[T any](params RunParams[T]) error {
	// Load plan from file
	var plan types.Plan[T]
	if err := utils.ParseJSONFile(params.PlanFilePath, "plan file", &plan); err != nil {
		fmt.Printf("%sfailed to load plan file: %v%s\n", constants.ColorRed, err, constants.ColorReset)
		return fmt.Errorf("failed to load plan file: %v", err)
	}

	if plan.IsEmpty() {
		fmt.Printf("%sNo changes required%s\n", constants.ColorGreen, constants.ColorReset)
		return nil
	}

	// Execute DB operations
	result, err := ExecuteOperations(ExecuteOperationsParams[T]{
		Plan:            plan,
		FormatRecord:    params.FormatRecord,
		FormatKey:       params.FormatKey,
		OnAdd:           params.OnAdd,
		OnUpdate:        params.OnUpdate,
		OnDelete:        params.OnDelete,
		OnFinalize:      params.OnFinalize,
		Parallelization: params.Parallelization,
	})

	// Always print execution report (even if operations or finalization failed)
	if result != nil {
		fmt.Print(formatters.FormatExecutionReport(*result, params.FormatRecord, params.FormatKey))
	}

	// Determine if there were any failures (operations or finalization)
	var finalErr error
	if err != nil {
		// Finalization failed
		finalErr = err
	} else if result != nil {
		// Check if any operations failed
		failureCount := len(result.Failure.Additions) + len(result.Failure.Updates) + len(result.Failure.Deletions)
		if failureCount > 0 {
			finalErr = fmt.Errorf("some operations failed: %d added, %d updated, %d deleted",
				len(result.Failure.Additions), len(result.Failure.Updates), len(result.Failure.Deletions))
		}
	}

	// Print error message if there were failures (defer ensures report is printed first)
	defer func() {
		if finalErr != nil {
			fmt.Printf("%s%s%s\n",
				constants.ColorRed,
				finalErr.Error(),
				constants.ColorReset)
		}
	}()

	return finalErr
}
