package formatters

import (
	"testing"

	"github.com/algebananazzzzz/planear/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRecord struct {
	ID   string
	Name string
}

func formatMockRecord(r mockRecord) string {
	return r.ID + ":" + r.Name
}

func formatMockKey(key string) string {
	return "[[" + key + "]]"
}

func TestFormatLegend(t *testing.T) {
	expected := "Actions are indicated with the following symbols:\n" +
		"    \x1b[32m+\x1b[0m add\n" +
		"    \x1b[33m~\x1b[0m update\n" +
		"    \x1b[31m-\x1b[0m delete\n" +
		"    \x1b[35m?\x1b[0m ignore\n\n"

	assert.Equal(t, expected, formatLegend())
}

func TestFormatValue(t *testing.T) {
	t.Run("non-pointer value", func(t *testing.T) {
		require.Equal(t, "hello", formatValue("hello"))
		require.Equal(t, 42, formatValue(42))
		require.Equal(t, true, formatValue(true))
	})

	t.Run("nil pointer", func(t *testing.T) {
		var s *string
		var i *int
		var b *bool

		require.Equal(t, "null", formatValue(s))
		require.Equal(t, "null", formatValue(i))
		require.Equal(t, "null", formatValue(b))
	})

	t.Run("non-nil pointer", func(t *testing.T) {
		str := "world"
		integer := 99
		boolean := false

		require.Equal(t, "world", formatValue(&str))
		require.Equal(t, 99, formatValue(&integer))
		require.Equal(t, false, formatValue(&boolean))
	})

	t.Run("nil interface", func(t *testing.T) {
		var v any = nil
		require.Equal(t, nil, formatValue(v)) // not a typed nil pointer
	})
}

func TestFormatAdd(t *testing.T) {
	rec := types.RecordAddition[mockRecord]{
		New: mockRecord{ID: "1", Name: "Alice"},
	}
	expected := "    \033[32m+\033[0m 1:Alice\n"
	assert.Equal(t, expected, FormatAdd(rec, formatMockRecord))
}

func TestFormatUpdate(t *testing.T) {
	rec := types.RecordUpdate[mockRecord]{
		Key: "key-123",
		Changes: []types.FieldChange{
			{Field: "Name", OldValue: "Bob", NewValue: "Bobby"},
			{Field: "Age", OldValue: "20", NewValue: "21"},
		},
	}
	expected := "    \033[33m~\033[0m [[key-123]], Name: Bob => Bobby, Age: 20 => 21\n"
	assert.Equal(t, expected, FormatUpdate(rec, formatMockKey))
}

func TestFormatDelete(t *testing.T) {
	rec := types.RecordDeletion[mockRecord]{
		Old: mockRecord{ID: "2", Name: "Charlie"},
	}
	expected := "    \033[31m-\033[0m 2:Charlie\n"
	assert.Equal(t, expected, FormatDelete(rec, formatMockRecord))
}

func TestFormatIgnore(t *testing.T) {
	rec := types.RecordIgnored[mockRecord]{
		Record: mockRecord{ID: "3", Name: "Diana"},
		Reason: "duplicate",
	}
	expected := "    \033[35m? duplicate:\033[0m 3:Diana\n"
	assert.Equal(t, expected, formatIgnore(rec, formatMockRecord))
}
