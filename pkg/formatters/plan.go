package formatters

import (
	"fmt"
	"strings"

	"github.com/algebananazzzzz/planear/pkg/types"
)

func FormatPlan[T any](
	plan types.Plan[T],
	formatRecord func(T) string,
	formatKey func(string) string,
) string {
	var b strings.Builder

	b.WriteString(formatLegend())

	b.WriteString("Executing plan will perform the following actions:\n")

	planDetails, planSummary := formatPlanDetails[T](plan, formatRecord, formatKey)

	b.WriteString(planDetails)

	fmt.Fprintf(&b, "\nSummary: %d to add, %d to update, %d to remove, %d to ignore. Total: %d actions.\n",
		planSummary.Addition, planSummary.Update, planSummary.Deletion, planSummary.Ignore, planSummary.Total)

	return b.String()
}
