// Package plan provides functionality to generate reconciliation plans.
//
// A reconciliation plan compares desired state (from local CSV files) with actual
// state (from a remote system like a database) and produces a structured plan
// describing what changes need to be made.
//
// # Plan Generation Workflow
//
// The Generate function performs the following steps:
//
// 1. Load local records from CSV directory
// 2. Load remote records via user-provided callback
// 3. Compare records to identify differences
// 4. Validate local records (skipping invalid ones)
// 5. Generate plan with field-level change details
// 6. Output plan as JSON and human-readable format
//
// # Usage
//
// To generate a plan, prepare GenerateParams with callbacks for your domain:
//
//	params := plan.GenerateParams[MyRecord]{
//	    CSVPath:    "./data",
//	    OutputFilePath: "./plan.json",
//
//	    // Extract primary key from record
//	    ExtractKeyFunc: func(r MyRecord) string {
//	        return r.ID
//	    },
//
//	    // Load current state from database
//	    LoadRemoteRecords: func() (map[string]MyRecord, error) {
//	        return loadFromDatabase()
//	    },
//
//	    // Optional: validate records
//	    ValidateRecord: func(r MyRecord) error {
//	        if r.Value < 0 {
//	            return errors.New("value cannot be negative")
//	        }
//	        return nil
//	    },
//
//	    // Format functions for display
//	    FormatRecordFunc: func(r MyRecord) string {
//	        return fmt.Sprintf("%s (value: %d)", r.ID, r.Value)
//	    },
//	    FormatKeyFunc: func(key string) string {
//	        return key
//	    },
//	}
//
//	plan, err := plan.Generate(params)
//	if err != nil {
//	    log.Fatalf("failed to generate plan: %v", err)
//	}
//
// # Plan Output
//
// The generated plan is saved as JSON and printed to stdout. It contains:
//
//   - Additions: New records to create
//   - Updates: Existing records to modify (with field-level changes)
//   - Deletions: Records to remove
//   - Ignores: Records that failed validation (with reason)
//
// # CSV Format
//
// CSV files should have headers matching struct field tags. Example for a struct:
//
//	type User struct {
//	    Email string  `csv:"email"`
//	    Name  string  `csv:"name"`
//	    Age   int     `csv:"age"`
//	}
//
// CSV file (users.csv):
//
//	email,name,age
//	alice@example.com,Alice,30
//	bob@example.com,Bob,25
//
// # Error Handling
//
// Generate returns an error if:
//   - CSV directory is invalid or files cannot be read
//   - LoadRemoteRecords callback fails
//   - Plan cannot be written to file
//
// Records that fail validation are not considered errors—they're added to the
// Ignores list in the plan with the validation error as reason.
//
// See package github.com/algebananazzzzz/planear/pkg/core/apply to execute plans.
package plan
