// Package apply provides functionality to execute reconciliation plans.
//
// After a plan is generated (see package github.com/algebananazzzzz/planear/pkg/core/plan),
// the apply package executes the plan by invoking user-defined callbacks for each operation.
//
// # Plan Execution Workflow
//
// The Run function performs the following steps:
//
// 1. Load plan from JSON file
// 2. Execute all operations in parallel with retry logic
// 3. Invoke callbacks for each operation (OnAdd, OnUpdate, OnDelete)
// 4. Handle failures and retry with exponential backoff
// 5. Execute finalization hook
// 6. Generate execution report tracking successes and failures
//
// # Usage
//
// To execute a plan, define RunParams with callbacks that perform your domain operations:
//
//	params := apply.RunParams[MyRecord]{
//	    PlanFilePath: "./plan.json",
//
//	    // Define what happens for each operation type
//	    OnAdd: func(addition types.RecordAddition[MyRecord]) error {
//	        // Insert into database
//	        return db.Insert(addition.New)
//	    },
//
//	    OnUpdate: func(update types.RecordUpdate[MyRecord]) error {
//	        // Update in database
//	        return db.Update(update.New)
//	    },
//
//	    OnDelete: func(deletion types.RecordDeletion[MyRecord]) error {
//	        // Delete from database
//	        return db.Delete(deletion.Key)
//	    },
//
//	    OnFinalize: func() error {
//	        // Optional: commit transaction, send notifications, etc.
//	        return db.Commit()
//	    },
//
//	    // Format functions for display
//	    FormatRecord: func(r MyRecord) string {
//	        return fmt.Sprintf("%s (value: %d)", r.ID, r.Value)
//	    },
//	    FormatKey: func(key string) string {
//	        return key
//	    },
//
//	    // Run up to 5 operations in parallel
//	    Parallelization: ptr(5),
//	}
//
//	if err := apply.Run(params); err != nil {
//	    log.Fatalf("failed to execute plan: %v", err)
//	}
//
// # Callbacks
//
// RunParams requires callbacks for each operation type:
//
// - OnAdd: Called for each RecordAddition. Should insert the new record.
// - OnUpdate: Called for each RecordUpdate. Should update the existing record.
// - OnDelete: Called for each RecordDeletion. Should delete the record.
// - OnFinalize: Called once after all operations complete. Optional for cleanup/commit.
//
// All callbacks receive full context about the operation including old/new values
// and field-level changes (for updates).
//
// # Parallel Execution
//
// By default, operations run serially. To execute in parallel:
//
//	parallelism := 5
//	params.Parallelization = &parallelism
//
// The library will execute up to 5 operations concurrently.
//
// # Retry Logic
//
// Failed operations are automatically retried with exponential backoff:
//
//   - First retry: 100ms delay
//	  - Second retry: 200ms delay
//	  - Third retry: 400ms delay
//	  - Further retries: exponential backoff up to reasonable limits
//
// After all retries are exhausted, the operation is marked as failed in the report.
//
// # Execution Report
//
// After execution completes, a report is generated showing:
//
//   - Success: Operations that completed successfully
//   - Failure: Operations that failed after retries
//   - Ignores: Records that were skipped during execution
//
// # Error Handling
//
// Run returns an error if:
//   - Plan file cannot be read
//   - There's a critical error loading the plan
//
// Individual operation failures are not returned as errors—they're tracked
// in the execution report instead. This allows partial execution to complete.
//
// # Example: Database Synchronization
//
// Here's a complete example of syncing a database from CSV:
//
//	type User struct {
//	    ID    string `csv:"id"`
//	    Name  string `csv:"name"`
//	    Email string `csv:"email"`
//	}
//
//	runParams := apply.RunParams[User]{
//	    PlanFilePath: "plan.json",
//
//	    OnAdd: func(add types.RecordAddition[User]) error {
//	        return db.InsertUser(add.New)
//	    },
//	    OnUpdate: func(upd types.RecordUpdate[User]) error {
//	        return db.UpdateUser(upd.New)
//	    },
//	    OnDelete: func(del types.RecordDeletion[User]) error {
//	        return db.DeleteUser(del.Old.ID)
//	    },
//	    OnFinalize: func() error {
//	        return db.Commit() // Commit the transaction
//	    },
//	    FormatRecord: func(u User) string {
//	        return fmt.Sprintf("%s <%s>", u.Name, u.Email)
//	    },
//	    FormatKey: func(key string) string {
//	        return key
//	    },
//	    Parallelization: ptr(10),
//	}
//
//	if err := apply.Run(runParams); err != nil {
//	    log.Fatalf("execution failed: %v", err)
//	}
//
// See package github.com/algebananazzzzz/planear/pkg/core/plan to generate plans.
package apply
