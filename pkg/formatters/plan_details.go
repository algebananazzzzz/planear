package formatters

import (
	"fmt"
	"strings"

	"github.com/algebananazzzzz/planear/pkg/types"
)

type PlanSummary struct {
	Addition int
	Update   int
	Deletion int
	Ignore   int
	Total    int
}

// formatPlanDetails generates a formatted summary string describing the actions
// to be performed in a given plan, including additions, updates, deletions, and ignores.
// It returns the formatted string and a PlanSummary containing counts for each action type.
//
// Parameters:
//   - plan: the reconciliation plan containing categorized actions
//   - formatRecord: function to format a record for output
//   - formatKey: function to format a record's key for output
//
// Returns:
//   - A human-readable string detailing the planned changes
//   - A PlanSummary with counts of additions, updates, deletions, and ignores
func formatPlanDetails[T any](
	plan types.Plan[T],
	formatRecord func(T) string,
	formatKey func(string) string,
) (string, PlanSummary) {
	var b strings.Builder

	if len(plan.Additions) > 0 {
		fmt.Fprintf(&b, "\n# %d row(s) will be added\n", len(plan.Additions))
		for _, a := range plan.Additions {
			fmt.Fprint(&b, FormatAdd(a, formatRecord))
		}
	}

	if len(plan.Updates) > 0 {
		fmt.Fprintf(&b, "\n# %d row(s) will be updated\n", len(plan.Updates))
		for _, u := range plan.Updates {
			fmt.Fprint(&b, FormatUpdate(u, formatKey))
		}
	}

	if len(plan.Deletions) > 0 {
		fmt.Fprintf(&b, "\n# %d row(s) will be deleted\n", len(plan.Deletions))
		for _, d := range plan.Deletions {
			fmt.Fprint(&b, FormatDelete(d, formatRecord))
		}
	}

	if len(plan.Ignores) > 0 {
		fmt.Fprintf(&b, "\n# %d row(s) will be ignored\n", len(plan.Ignores))
		for _, i := range plan.Ignores {
			fmt.Fprint(&b, formatIgnore(i, formatRecord))
		}
	}

	return b.String(), PlanSummary{
		Addition: len(plan.Additions),
		Update:   len(plan.Updates),
		Deletion: len(plan.Deletions),
		Ignore:   len(plan.Ignores),
		Total:    len(plan.Additions) + len(plan.Updates) + len(plan.Deletions) + len(plan.Ignores),
	}
}

// For testing purposes only: expose the private formatPlanDetails function.
func TestableFormatPlanDetails[T any](
	plan types.Plan[T],
	formatRecord func(T) string,
	formatKey func(string) string,
) (string, PlanSummary) {
	return formatPlanDetails(plan, formatRecord, formatKey)
}
