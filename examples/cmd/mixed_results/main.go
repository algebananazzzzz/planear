package main

import (
	"errors"
	"os"
	"sync"

	"github.com/algebananazzzzz/planear/examples/lib"
	"github.com/algebananazzzzz/planear/pkg/core/apply"
	"github.com/algebananazzzzz/planear/pkg/types"
)

// Track attempts for mixed failure simulation
var (
	addAttempts    = make(map[string]int)
	updateAttempts = make(map[string]int)
	attemptMutex   sync.Mutex
)

// addWithMixedResults demonstrates different failure patterns:
// - user1: succeeds immediately (no retries)
// - user2: fails once, then succeeds
// - user3: fails twice, then succeeds
// - user4: always fails (fails all 3 retries)
// - user5: succeeds immediately
func addWithMixedResults(rec types.RecordAddition[lib.UserRecord]) error {
	attemptMutex.Lock()
	addAttempts[rec.New.Email]++
	attempt := addAttempts[rec.New.Email]
	attemptMutex.Unlock()

	switch rec.New.Email {
	case "user1@example.com":
		return nil
	case "user5@example.com":
		if attempt == 1 {
			return errors.New("network timeout")
		}
		return nil
	case "user9@example.com":
		if attempt <= 2 {
			return errors.New("database connection lost")
		}
		return nil
	case "user10@example.com":
		return errors.New("permission denied")
	}
	return nil
}

// updateWithMixedResults demonstrates different update failure patterns
func updateWithMixedResults(rec types.RecordUpdate[lib.UserRecord]) error {
	attemptMutex.Lock()
	updateAttempts[rec.New.Email]++
	attempt := updateAttempts[rec.New.Email]
	attemptMutex.Unlock()

	switch rec.New.Email {
	case "user2@example.com":
		if attempt == 1 {
			return errors.New("lock timeout")
		}
		return nil
	case "user7@example.com":
		return errors.New("validation failed")
	}
	return nil
}

func main() {
	// Apply the plan with custom callbacks that simulate mixed successes and failures
	// Uses the standard apply entrypoint - all formatting is handled by pkg
	if err := apply.Run(apply.RunParams[lib.UserRecord]{
		PlanFilePath: lib.OutputPlanPath,
		FormatRecord: lib.FormatRecord,
		FormatKey:    lib.FormatKey,
		OnAdd:        addWithMixedResults,
		OnUpdate:     updateWithMixedResults,
		OnDelete:     lib.DeleteUser,
		OnFinalize:   lib.Finalize,
	}); err != nil {
		os.Exit(1)
	}
}
