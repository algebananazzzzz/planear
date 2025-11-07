package formatters_test

import (
	"fmt"
	"testing"

	"github.com/algebananazzzzz/planear/pkg/formatters"
	"github.com/algebananazzzzz/planear/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestFormatExecutionReport(t *testing.T) {
	report := types.ExecutionReport[MockRecord]{
		Success: types.Plan[MockRecord]{
			Additions: []types.RecordAddition[MockRecord]{
				{Key: "1", New: MockRecord{ID: "1", Name: "Alice"}},
			},
			Updates: []types.RecordUpdate[MockRecord]{
				{
					Key: "2",
					Old: MockRecord{ID: "2", Name: "Bob"},
					New: MockRecord{ID: "2", Name: "Robert"},
					Changes: []types.FieldChange{
						{Field: "Name", OldValue: "Bob", NewValue: "Robert"},
					},
				},
			},
			Deletions: []types.RecordDeletion[MockRecord]{
				{Key: "3", Old: MockRecord{ID: "3", Name: "Charlie"}},
			},
		},
		Failure: types.Plan[MockRecord]{
			Additions: []types.RecordAddition[MockRecord]{
				{Key: "4", New: MockRecord{ID: "4", Name: "David"}},
			},
		},
		Ignores: []types.RecordIgnored[MockRecord]{
			{Record: MockRecord{ID: "5", Name: "Eve"}, Reason: "Already exists"},
		},
	}

	output := formatters.FormatExecutionReport(report, formatMockRecord, formatMockKey)

	assert.Contains(t, output, "Actions are indicated with the following symbols:")
	assert.Contains(t, output, fmt.Sprintf("%s+%s add", ColorGreen, ColorReset))
	assert.Contains(t, output, "# 3 operation(s) succeeded")
	assert.Contains(t, output, "ID=1 Name=Alice")
	assert.Contains(t, output, "Name: Bob => Robert")
	assert.Contains(t, output, "ID=3 Name=Charlie")
	assert.Contains(t, output, "Summary: 1 added, 1 updated, 1 deleted")

	assert.Contains(t, output, "# 1 operation(s) failed")
	assert.Contains(t, output, "ID=4 Name=David")
	assert.Contains(t, output, "Summary: 1 added, 0 updated, 0 deleted")

	assert.Contains(t, output, "# 1 entrie(s) were ignored")
	assert.Contains(t, output, fmt.Sprintf("%s? Already exists:%s ID=5 Name=Eve", ColorPurple, ColorReset))

	assert.Contains(t, output, "Execution result: 3 succeeded, 1 failed, 1 ignored")
}
