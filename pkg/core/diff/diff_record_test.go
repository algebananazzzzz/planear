package diff_test

import (
	"testing"

	"github.com/algebananazzzzz/planear/pkg/core/diff"
	"github.com/algebananazzzzz/planear/pkg/types"
	"github.com/stretchr/testify/require"
)

type SimpleRecord struct {
	ID    int    `csv:"id"`
	Name  string `csv:"name"`
	Email string `csv:"email"`
	Skip  string `csv:"-"` // should be ignored
}

type PointerRecord struct {
	ID    *int    `csv:"id"`
	Email *string `csv:"email"`
}

func TestDiffRecords_Primitives(t *testing.T) {
	old := SimpleRecord{ID: 1, Name: "Alice", Email: "alice@example.com"}
	new := SimpleRecord{ID: 2, Name: "Alice", Email: "bob@example.com"}

	changes, err := diff.DiffRecords(old, new)
	require.NoError(t, err)
	require.ElementsMatch(t, []types.FieldChange{
		{Field: "id", OldValue: 1, NewValue: 2},
		{Field: "email", OldValue: "alice@example.com", NewValue: "bob@example.com"},
	}, changes)
}

func TestDiffRecords_NoChanges(t *testing.T) {
	r := SimpleRecord{ID: 1, Name: "Same", Email: "same@example.com"}
	changes, err := diff.DiffRecords(r, r)
	require.NoError(t, err)
	require.Empty(t, changes)
}

func TestDiffRecords_TypeMismatch(t *testing.T) {
	type A struct {
		Field int `csv:"field"`
	}

	oldVal := A{Field: 1}
	newVal := &A{Field: 1} // pointer to A

	_, err := diff.DiffRecords[any](oldVal, newVal)
	require.Error(t, err)
	require.Contains(t, err.Error(), "type mismatch")
}

func TestDiffRecords_UnexportedAndIgnoredFields(t *testing.T) {
	type Mixed struct {
		Public  string `csv:"public"`
		private string `csv:"private"` // unexported
		Skip    string `csv:"-"`       // ignored
	}

	old := Mixed{Public: "a", private: "x", Skip: "x"}
	new := Mixed{Public: "b", private: "y", Skip: "y"}

	changes, err := diff.DiffRecords(old, new)
	require.NoError(t, err)
	require.Len(t, changes, 1)
	require.Equal(t, "public", changes[0].Field)
	require.Equal(t, "a", changes[0].OldValue)
	require.Equal(t, "b", changes[0].NewValue)
}

func TestDiffRecords_PointerFieldChange(t *testing.T) {
	v1, v2 := 1, 2
	email1, email2 := "a@example.com", "b@example.com"

	old := PointerRecord{ID: &v1, Email: &email1}
	new := PointerRecord{ID: &v2, Email: &email2}

	changes, err := diff.DiffRecords(old, new)
	require.NoError(t, err)
	require.ElementsMatch(t, []types.FieldChange{
		{Field: "id", OldValue: &v1, NewValue: &v2},
		{Field: "email", OldValue: &email1, NewValue: &email2},
	}, changes)
}

func TestDiffRecords_PointerField_NilChanges(t *testing.T) {
	v := 100
	email := "test@example.com"

	var nilIntPtr *int = nil
	var nilStringPtr *string = nil

	// Case 1: nil → value
	old1 := PointerRecord{ID: nilIntPtr, Email: nilStringPtr}
	new1 := PointerRecord{ID: &v, Email: &email}

	changes1, err1 := diff.DiffRecords(old1, new1)
	require.NoError(t, err1)
	require.ElementsMatch(t, []types.FieldChange{
		{Field: "id", OldValue: nilIntPtr, NewValue: &v},
		{Field: "email", OldValue: nilStringPtr, NewValue: &email},
	}, changes1)

	// Case 2: value → nil
	old2 := PointerRecord{ID: &v, Email: &email}
	new2 := PointerRecord{ID: nilIntPtr, Email: nilStringPtr}

	changes2, err2 := diff.DiffRecords(old2, new2)
	require.NoError(t, err2)
	require.ElementsMatch(t, []types.FieldChange{
		{Field: "id", OldValue: &v, NewValue: nilIntPtr},
		{Field: "email", OldValue: &email, NewValue: nilStringPtr},
	}, changes2)
}
