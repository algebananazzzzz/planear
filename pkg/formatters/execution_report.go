package formatters

import (
	"fmt"
	"strings"

	"github.com/algebananazzzzz/planear/pkg/constants"
	"github.com/algebananazzzzz/planear/pkg/types"
)

// FormatExecutionReport formats the result of executing a migration or transformation plan,
// grouping results into successful operations, failures, and ignored entries.
//
// The report includes:
// - A legend of action symbols (add, update, remove, ignore).
// - A summary of successfully executed changes with a count and per-record details.
// - A summary of failed operations, similarly detailed.
// - Any ignored entries with explanations.
//
// Parameters:
//   - result: an ExecutionReport containing slices of successful, failed, and ignored changes.
//   - formatRecord: a function to format individual record values.
//   - formatKey: a function to format record keys.
//
// Returns:
//
//	A string representing a human-readable, color-coded report.
func FormatExecutionReport[T any](
	result types.ExecutionReport[T],
	formatRecord func(T) string,
	formatKey func(string) string,
) string {
	var b strings.Builder
	success := result.Success
	failure := result.Failure
	ignores := result.Ignores

	// Explain legend
	b.WriteString("Actions are indicated with the following symbols:\n")
	fmt.Fprintf(&b, "    %s+%s add\n", constants.ColorGreen, constants.ColorReset)
	fmt.Fprintf(&b, "    %s~%s update\n", constants.ColorYellow, constants.ColorReset)
	fmt.Fprintf(&b, "    %s-%s remove\n", constants.ColorRed, constants.ColorReset)
	fmt.Fprintf(&b, "    %s?%s ignore\n\n", constants.ColorPurple, constants.ColorReset)

	b.WriteString("Summary of the executed result:\n")

	successReport, successCount := formatPlanDetails(success, formatRecord, formatKey)
	failureReport, failureCount := formatPlanDetails(failure, formatRecord, formatKey)

	fmt.Fprintf(&b, "\n# %d operation(s) succeeded\n", successCount.Total)
	fmt.Fprint(&b, successReport)
	fmt.Fprintf(&b, "Summary: %d added, %d updated, %d deleted\n", successCount.Addition, successCount.Update, successCount.Deletion)

	fmt.Fprintf(&b, "\n# %d operation(s) failed\n", failureCount.Total)
	fmt.Fprint(&b, failureReport)
	fmt.Fprintf(&b, "Summary: %d added, %d updated, %d deleted\n", failureCount.Addition, failureCount.Update, failureCount.Deletion)

	if len(ignores) > 0 {
		fmt.Fprintf(&b, "\n# %d entrie(s) were ignored\n", len(ignores))
		for _, i := range ignores {
			fmt.Fprint(&b, formatIgnore(i, formatRecord))
		}
	}

	// Finalization status
	b.WriteString("\nFinalization:\n")
	if result.FinalizationSuccess {
		fmt.Fprintf(&b, "  %s✓ Finalization succeeded%s\n", constants.ColorGreen, constants.ColorReset)
	} else {
		fmt.Fprintf(&b, "  %s✗ Finalization failed: %s%s\n", constants.ColorRed, result.FinalizationErrorMsg, constants.ColorReset)
	}

	fmt.Fprintf(&b, "\nExecution result: %d succeeded, %d failed, %d ignored\n", successCount.Total, failureCount.Total, len(ignores))
	return b.String()
}
