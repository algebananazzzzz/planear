// Package types defines the core data structures used throughout Planear.
//
// This package provides the fundamental types for reconciliation planning and execution:
//
// # Plans
//
// A Plan[T] represents a set of changes to be applied to reconcile local desired state
// with remote actual state. It contains four types of operations:
//
//   - Additions: New records to be created (in local but not in remote)
//   - Updates: Existing records to be modified (in both but with different values)
//   - Deletions: Records to be removed (in remote but not in local)
//   - Ignores: Local records that could not be used (failed validation, etc.)
//
// # Record Operations
//
// Each operation type carries both the record data and metadata:
//
//   - RecordAddition: The new record to create
//   - RecordUpdate: Both old and new values, plus field-level changes
//   - RecordDeletion: The old record being removed
//   - RecordIgnored: The record that was skipped, with reason
//
// # Execution Reports
//
// An ExecutionReport[T] tracks the results of executing a plan:
//
//   - Success: Operations that completed successfully
//   - Failure: Operations that failed (may be retried)
//   - Ignores: Records that were skipped during execution
//
// # Example Usage
//
// Define a record type and work with plans:
//
//	type User struct {
//	    Email string
//	    Name  string
//	}
//
//	// Create a plan
//	plan := &types.Plan[User]{
//	    Additions: []types.RecordAddition[User]{
//	        {
//	            Key: "alice@example.com",
//	            New: User{Email: "alice@example.com", Name: "Alice"},
//	        },
//	    },
//	}
//
//	// Check if plan is empty
//	if plan.IsEmpty() {
//	    fmt.Println("No changes needed")
//	}
//
// See package github.com/algebananazzzzz/planear/pkg/core/plan for generating plans
// and package github.com/algebananazzzzz/planear/pkg/core/apply for executing them.
package types
