package diff

import (
	"fmt"
	"reflect"

	"github.com/algebananazzzzz/planear/pkg/types"
)

// ComputePlanDiff compares local and remote records to generate a reconciliation plan.
// It identifies which records need to be added, updated, removed, or ignored.
// Validation errors on local records cause them to be ignored with the associated reason.
//
// Parameters:
//   - localRecords: map of current local records keyed by string
//   - remoteRecords: map of existing remote records keyed by string
//   - validator: function to validate a local record; returns error if invalid
//
// Returns:
//   - a Plan containing additions, updates, deletions, and ignores for reconciliation
//   - error if diffing encounters unexpected failures (e.g., during update diff generation)
func ComputePlanDiff[T any](
	localRecords, remoteRecords map[string]T,
	validator func(T) error,
) (types.Plan[T], error) {
	var (
		additions []types.RecordAddition[T]
		updates   []types.RecordUpdate[T]
		deletions []types.RecordDeletion[T]
		ignores   []types.RecordIgnored[T]
	)

	processedKeys := make(map[string]bool)

	for key, localRecord := range localRecords {
		processedKeys[key] = true

		// Validate local record; ignore if invalid
		if err := validator(localRecord); err != nil {
			ignores = append(ignores, types.RecordIgnored[T]{
				Key:    key,
				Record: localRecord,
				Reason: err.Error(),
			})
			continue
		}

		remoteRecord, exists := remoteRecords[key]
		if exists {
			// Compare local vs remote; if different, compute field-level changes
			if !reflect.DeepEqual(localRecord, remoteRecord) {
				changes, err := DiffRecords(remoteRecord, localRecord)
				if err != nil {
					return types.Plan[T]{}, fmt.Errorf("error generating update diff for key %q: %w", key, err)
				}
				updates = append(updates, types.RecordUpdate[T]{
					Key:     key,
					Changes: changes,
					Old:     remoteRecord,
					New:     localRecord,
				})
			}
		} else {
			// Key not present remotely, so add as new record
			additions = append(additions, types.RecordAddition[T]{
				Key: key,
				New: localRecord,
			})
		}
	}

	// Keys in remote but not in local are deletions
	for key, remoteRecord := range remoteRecords {
		if !processedKeys[key] {
			deletions = append(deletions, types.RecordDeletion[T]{
				Key: key,
				Old: remoteRecord,
			})
		}
	}

	return types.Plan[T]{
		Additions: additions,
		Updates:   updates,
		Deletions: deletions,
		Ignores:   ignores,
	}, nil
}
