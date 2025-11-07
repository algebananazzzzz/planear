package diff_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/algebananazzzzz/planear/pkg/core/diff"
)

// Simple test struct with csv tags for diffing
type TestRecord struct {
	ID    string  `csv:"id"`
	Name  string  `csv:"name"`
	Count *int    `csv:"count"`
	Email *string `csv:"email"`
}

func validatorWithError(r TestRecord) error {
	if r.Name == "bad" {
		return errors.New("validation error")
	}
	return nil
}

func TestComputePlanDiff(t *testing.T) {
	// Prepare test data
	count1, count2 := 10, 20
	email1, email2 := "a@example.com", "b@example.com"

	local := map[string]TestRecord{
		"1": {ID: "1", Name: "Alice", Count: &count1, Email: &email1},       // add (no remote)
		"2": {ID: "2", Name: "Bob Updated", Count: &count2, Email: &email2}, // update from remote
		"3": {ID: "3", Name: "bad", Count: nil, Email: nil},                 // ignore (validator error)
	}

	remote := map[string]TestRecord{
		"2": {ID: "2", Name: "Bob", Count: &count1, Email: &email1},
		"4": {ID: "4", Name: "Deleted", Count: nil, Email: nil}, // delete (not in local)
	}

	plan, err := diff.ComputePlanDiff(local, remote, validatorWithError)
	require.NoError(t, err)

	// Validate additions
	require.Len(t, plan.Additions, 1)
	require.Equal(t, "1", plan.Additions[0].Key)
	require.Equal(t, "Alice", plan.Additions[0].New.Name)

	// Validate updates
	require.Len(t, plan.Updates, 1)
	require.Equal(t, "2", plan.Updates[0].Key)
	foundNameChange := false
	foundCountChange := false
	foundEmailChange := false
	for _, c := range plan.Updates[0].Changes {
		switch c.Field {
		case "name":
			require.Equal(t, "Bob", c.OldValue)
			require.Equal(t, "Bob Updated", c.NewValue)
			foundNameChange = true
		case "count":
			require.Equal(t, &count1, c.OldValue)
			require.Equal(t, &count2, c.NewValue)
			foundCountChange = true
		case "email":
			require.Equal(t, &email1, c.OldValue)
			require.Equal(t, &email2, c.NewValue)
			foundEmailChange = true
		}
	}
	require.True(t, foundNameChange, "Expected name field change")
	require.True(t, foundCountChange, "Expected count field change")
	require.True(t, foundEmailChange, "Expected email field change")

	// Validate deletions
	require.Len(t, plan.Deletions, 1)
	require.Equal(t, "4", plan.Deletions[0].Key)
	require.Equal(t, "Deleted", plan.Deletions[0].Old.Name)

	// Validate ignores (validation error)
	require.Len(t, plan.Ignores, 1)
	require.Equal(t, "3", plan.Ignores[0].Key)
	require.Equal(t, "validation error", plan.Ignores[0].Reason)
}

type Record struct {
	ID string `csv:"id"`
}
