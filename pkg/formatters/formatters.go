package formatters

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/algebananazzzzz/planear/pkg/constants"
	"github.com/algebananazzzzz/planear/pkg/types"
)

// formatLegend returns a static legend explaining the symbols used for plan actions.
func formatLegend() string {
	return fmt.Sprintf(`Actions are indicated with the following symbols:
    %s+%s add
    %s~%s update
    %s-%s delete
    %s?%s ignore

`, constants.ColorGreen, constants.ColorReset,
		constants.ColorYellow, constants.ColorReset,
		constants.ColorRed, constants.ColorReset,
		constants.ColorPurple, constants.ColorReset)
}

// formatValue returns "null" if the value is a nil pointer,
// otherwise the dereferenced value (for pointers) or the value itself.
func formatValue(v any) any {
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return "null"
		}
		return val.Elem().Interface()
	}
	return v
}

// FormatAdd returns a formatted string representing a record addition.
func FormatAdd[T any](record types.RecordAddition[T], formatRecord func(T) string) string {
	return fmt.Sprintf("    %s+%s %s\n",
		constants.ColorGreen,
		constants.ColorReset,
		formatRecord(record.New),
	)
}

// formatUpdate returns a formatted string representing the changes in a record update.
func FormatUpdate[T any](record types.RecordUpdate[T], formatKey func(string) string) string {
	var parts []string
	for _, change := range record.Changes {
		oldVal := formatValue(change.OldValue)
		newVal := formatValue(change.NewValue)
		parts = append(parts, fmt.Sprintf("%s: %v => %v", change.Field, oldVal, newVal))
	}
	updates := strings.Join(parts, ", ")

	return fmt.Sprintf("    %s~%s %s, %s\n",
		constants.ColorYellow,
		constants.ColorReset,
		formatKey(record.Key),
		updates)
}

// FormatDelete returns a formatted string representing a record deletion.
func FormatDelete[T any](record types.RecordDeletion[T], formatRecord func(T) string) string {
	return fmt.Sprintf("    %s-%s %s\n",
		constants.ColorRed,
		constants.ColorReset,
		formatRecord(record.Old),
	)
}

// formatIgnore returns a formatted string for an ignored record with its reason.
func formatIgnore[T any](record types.RecordIgnored[T], formatRecord func(T) string) string {
	return fmt.Sprintf("    %s? %s:%s %s\n",
		constants.ColorPurple,
		record.Reason,
		constants.ColorReset,
		formatRecord(record.Record),
	)
}
