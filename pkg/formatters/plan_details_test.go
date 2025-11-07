package formatters_test

import (
	"fmt"
	"testing"

	"github.com/algebananazzzzz/planear/pkg/formatters"
	"github.com/algebananazzzzz/planear/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestFormatPlanDetails(t *testing.T) {
	plan := types.Plan[MockRecord]{
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
		Ignores: []types.RecordIgnored[MockRecord]{
			{Record: MockRecord{ID: "4", Name: "Daisy"}, Reason: "no-op"},
		},
	}

	formatRecord := func(r MockRecord) string {
		return fmt.Sprintf("ID=%s Name=%s", r.ID, r.Name)
	}

	formatKey := func(key string) string {
		return fmt.Sprintf("[\x1b[33m%s\x1b[0m]", key)
	}

	output, summary := formatters.TestableFormatPlanDetails(plan, formatRecord, formatKey)

	expectedSummary := formatters.PlanSummary{
		Addition: 1,
		Update:   1,
		Deletion: 1,
		Ignore:   1,
		Total:    4,
	}
	assert.Equal(t, expectedSummary, summary)

	for _, line := range []string{
		"# 1 row(s) will be added",
		"\x1b[32m+\x1b[0m ID=1 Name=Alice",
		"# 1 row(s) will be updated",
		"[\x1b[33m2\x1b[0m], Name: Bob => Robert",
		"# 1 row(s) will be deleted",
		"\x1b[31m-\x1b[0m ID=3 Name=Charlie",
		"# 1 row(s) will be ignored",
		"\x1b[35m? no-op:\x1b[0m ID=4 Name=Daisy",
	} {
		assert.Contains(t, output, line)
	}
}
