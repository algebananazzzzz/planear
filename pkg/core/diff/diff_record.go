package diff

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/algebananazzzzz/planear/pkg/types"
)

// DiffRecords compares two records of the same struct type and returns a list of changed fields.
// Fields must be exported and tagged with `csv:"<name>"` to be considered.
func DiffRecords[T any](oldVal, newVal T) ([]types.FieldChange, error) {
	oldV := reflect.ValueOf(oldVal)
	newV := reflect.ValueOf(newVal)

	if oldV.Type() != newV.Type() {
		return nil, fmt.Errorf("type mismatch: %T vs %T", oldVal, newVal)
	}

	var changes []types.FieldChange
	t := oldV.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		if !field.IsExported() {
			continue
		}

		csvTag := field.Tag.Get("csv")
		fieldName := strings.Split(csvTag, ",")[0]
		if fieldName == "" || fieldName == "-" {
			continue
		}

		oldField := oldV.Field(i)
		newField := newV.Field(i)

		// Use DeepEqual to account for pointer values and nested structs
		if !reflect.DeepEqual(oldField.Interface(), newField.Interface()) {
			changes = append(changes, types.FieldChange{
				Field:    fieldName,
				OldValue: oldField.Interface(),
				NewValue: newField.Interface(),
			})
		}
	}

	return changes, nil
}
