package formatters_test

import (
	"fmt"
	"testing"

	"github.com/algebananazzzzz/planear/pkg/formatters"
	"github.com/algebananazzzzz/planear/pkg/types"
	"github.com/stretchr/testify/assert"
)

const (
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorRed    = "\033[31m"
	ColorPurple = "\033[35m"
	ColorReset  = "\033[0m"
)

type MockRecord struct {
	ID   string
	Name string
}

func formatMockRecord(r MockRecord) string {
	return fmt.Sprintf("ID=%s Name=%s", r.ID, r.Name)
}

func formatMockKey(key string) string {
	return fmt.Sprintf("[%s]", key)
}

func TestFormatPlan_EmptyPlan(t *testing.T) {
	plan := types.Plan[MockRecord]{}

	result := formatters.FormatPlan(plan, formatMockRecord, formatMockKey)

	assert.Contains(t, result, ColorGreen+"+"+ColorReset, "legend add symbol missing")
	assert.Contains(t, result, "Executing plan will perform the following actions:", "missing plan execution intro")

	expectedSummary := "Summary: 0 to add, 0 to update, 0 to remove, 0 to ignore. Total: 0 actions."
	assert.Contains(t, result, expectedSummary, "summary missing or incorrect")
}

func TestFormatPlan_AllActions(t *testing.T) {
	plan := types.Plan[MockRecord]{
		Additions: []types.RecordAddition[MockRecord]{
			{Key: "1", New: MockRecord{ID: "1", Name: "Alice"}},
		},
		Updates: []types.RecordUpdate[MockRecord]{
			{
				Key: "2",
				Changes: []types.FieldChange{
					{Field: "Name", OldValue: "Bob", NewValue: "Bobby"},
				},
				Old: MockRecord{ID: "2", Name: "Bob"},
				New: MockRecord{ID: "2", Name: "Bobby"},
			},
		},
		Deletions: []types.RecordDeletion[MockRecord]{
			{Key: "3", Old: MockRecord{ID: "3", Name: "Charlie"}},
		},
		Ignores: []types.RecordIgnored[MockRecord]{
			{Record: MockRecord{ID: "4", Name: "David"}, Reason: "No changes"},
		},
	}

	result := formatters.FormatPlan(plan, formatMockRecord, formatMockKey)

	assert.Contains(t, result, ColorGreen+"+", "add symbol/color missing")
	assert.Contains(t, result, ColorYellow+"~", "update symbol/color missing")
	assert.Contains(t, result, ColorRed+"-", "delete symbol/color missing")
	assert.Contains(t, result, ColorPurple+"?", "ignore symbol/color missing")

	assert.Contains(t, result, "Alice", "added record not formatted properly")
	assert.Contains(t, result, "[2]", "updated record key formatting missing")
	assert.Contains(t, result, "Charlie", "deleted record not formatted properly")
	assert.Contains(t, result, "David", "ignored record not formatted properly")

	expectedSummary := "Summary: 1 to add, 1 to update, 1 to remove, 1 to ignore. Total: 4 actions."
	assert.Contains(t, result, expectedSummary, "summary missing or incorrect")
}
