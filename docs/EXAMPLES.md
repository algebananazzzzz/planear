# Examples & Usage Patterns

This guide walks through the example implementation included in the repository and shows common usage patterns.

## Example Overview: User Management

The `/examples` directory contains a complete working example of managing user records with Planear.

**Scenario:** A system needs to keep user records synchronized between a CSV source (source of truth) and a database (actual state).

**What the example does:**
- Loads user records from `users.csv`
- Simulates loading existing users from a database
- Compares the two and generates a reconciliation plan
- Executes the plan with mock actions (prints instead of real DB calls)

## Project Structure

```
examples/
├── cmd/
│   ├── apply/
│   │   └── main.go          # Execute a generated plan
│   └── plan/
│       └── main.go          # Generate a reconciliation plan
├── lib/
│   ├── types.go             # UserRecord definition
│   ├── validators.go        # Validation logic
│   ├── remote.go            # Mock database loader
│   ├── actions.go           # Add/Update/Delete handlers
│   ├── formatters.go        # Display formatting
│   └── constants.go         # Configuration paths
└── data/
    └── users.csv            # Source CSV file
```

## The Data Model

Users are defined with these fields:

```go
type UserRecord struct {
    Email         string  `csv:"email"`        // Primary key
    Name          string  `csv:"name"`
    Points        int     `csv:"points"`
    DemeritPoints *int    `csv:"demerit_points"`  // Optional field
    ProfilePhoto  *string `csv:"profile_photo"`   // Optional field
}
```

**CSV Format:**

```csv
email,name,points,demerit_points,profile_photo
user1@example.com,Alice,100,,https://example.com/photo1.jpg
user2@example.com,Bob,60,2,https://example.com/photo2.jpg
```

Notes:
- Empty cells in the CSV become `nil` for pointer types (`*int`, `*string`)
- `email` is the primary key (extracted via `ExtractKey()`)
- Numeric fields are parsed as integers

## Validation Rules

The validator checks each record:

```go
func ValidateUserRecord(u UserRecord) error {
    if u.Points < 0 {
        return fmt.Errorf("points cannot be negative")
    }
    if u.DemeritPoints != nil && *u.DemeritPoints < 0 {
        return fmt.Errorf("demerit points cannot be negative")
    }
    if u.ProfilePhoto != nil && *u.ProfilePhoto != "" {
        _, err := url.ParseRequestURI(*u.ProfilePhoto)
        if err != nil {
            return fmt.Errorf("invalid profile photo URL")
        }
    }
    return nil
}
```

**Records that fail validation are ignored** (not added to the plan). You see them in the `Ignores` list with the validation error as the reason.

## Running the Example

### Step 1: Generate a Plan

```bash
cd examples/cmd/plan
go run main.go
```

Output shows:
- **Additions**: Records in CSV but not in DB
- **Updates**: Records in both but with differences
- **Deletions**: Records in DB but not in CSV
- **Ignores**: Records that failed validation
- Generated plan saved to `examples/data/generated/plan.json`

### Step 2: Inspect the Plan

The generated `plan.json` contains the complete plan in JSON format. Example structure:

```json
{
  "additions": [
    {
      "key": "user5@example.com",
      "new": {
        "email": "user5@example.com",
        "name": "Eve",
        "points": 120,
        "demerit_points": null,
        "profile_photo": "https://example.com/photo5.jpg"
      }
    }
  ],
  "updates": [
    {
      "key": "user2@example.com",
      "changes": [
        {
          "field": "points",
          "old_value": 50,
          "new_value": 60
        },
        {
          "field": "demerit_points",
          "old_value": 4,
          "new_value": 2
        }
      ],
      "old": { "email": "user2@example.com", "name": "Bob", "points": 50, ... },
      "new": { "email": "user2@example.com", "name": "Bob", "points": 60, ... }
    }
  ],
  "deletions": [
    {
      "key": "user7@example.com",
      "old": { ... }
    }
  ],
  "ignores": [
    {
      "key": "user4@example.com",
      "record": { ... },
      "reason": "invalid profile photo URL"
    }
  ]
}
```

### Step 3: Apply the Plan

```bash
cd examples/cmd/apply
go run main.go
```

Output shows:
- `[ADD]` for each addition
- `[UPDATE]` for each update with before/after
- `[DELETE]` for each deletion
- `[FINALIZE]` when complete

## Real-World Use Cases

### Use Case 1: User Management from LDAP/AD Sync

**Scenario:** Sync users from your LDAP directory to the database via CSV export.

```go
type ADUser struct {
    Username    string `csv:"username"`  // Primary key
    Email       string `csv:"email"`
    Department  string `csv:"department"`
    PhoneNumber string `csv:"phone"`
    IsActive    bool   `csv:"is_active"`
}

// Pull from LDAP, export to CSV, then reconcile
params := plan.GenerateParams[ADUser]{
    CSVPath: "ldap_export.csv",
    LoadRemoteRecords: func() (map[string]ADUser, error) {
        return db.FetchAllUsers()  // Current DB state
    },
    ValidateRecord: func(u ADUser) error {
        if u.Email == "" {
            return errors.New("email required")
        }
        if !strings.Contains(u.Email, "@company.com") {
            return errors.New("must be company email")
        }
        return nil
    },
    OnAdd: func(add types.RecordAddition[ADUser]) error {
        user := add.New
        hashedPwd := generateTempPassword()
        return db.CreateUser(user.Username, user.Email, hashedPwd)
    },
    OnUpdate: func(upd types.RecordUpdate[ADUser]) error {
        return db.UpdateUser(upd.New)
    },
    OnDelete: func(del types.RecordDeletion[ADUser]) error {
        // Mark inactive instead of deleting
        return db.DeactivateUser(del.Key)
    },
}
```

### Use Case 2: Database Permissions & Roles

**Scenario:** Sync role assignments from CSV to a role-based access control system.

```go
type RoleAssignment struct {
    UserID    string `csv:"user_id"`
    RoleName  string `csv:"role_name"`
}

// Composite key
func getCompositeKey(r RoleAssignment) string {
    return fmt.Sprintf("%s:%s", r.UserID, r.RoleName)
}

params := plan.GenerateParams[RoleAssignment]{
    ExtractKeyFunc: getCompositeKey,
    LoadRemoteRecords: func() (map[string]RoleAssignment, error) {
        // Query all current role assignments
        roles, err := rbac.GetAllRoleAssignments()
        // Convert to map with composite keys
        roleMap := make(map[string]RoleAssignment)
        for _, role := range roles {
            key := getCompositeKey(role)
            roleMap[key] = role
        }
        return roleMap, err
    },
    OnAdd: func(add types.RecordAddition[RoleAssignment]) error {
        return rbac.AssignRole(add.New.UserID, add.New.RoleName)
    },
    OnDelete: func(del types.RecordDeletion[RoleAssignment]) error {
        return rbac.RevokeRole(del.Key, del.Old.RoleName)
    },
}
```

### Use Case 3: Multi-Tenant Configuration Sync

**Scenario:** Manage configurations for multiple tenants from a single CSV.

```go
type TenantConfig struct {
    TenantID    string `csv:"tenant_id"`
    ConfigKey   string `csv:"config_key"`
    ConfigValue string `csv:"config_value"`
}

func getTenantConfigKey(tc TenantConfig) string {
    return fmt.Sprintf("%s/%s", tc.TenantID, tc.ConfigKey)
}

// Sync configs across all tenants in parallel
params := apply.RunParams[TenantConfig]{
    OnAdd: func(add types.RecordAddition[TenantConfig]) error {
        tenant := add.New.TenantID
        return configService.SetForTenant(tenant, add.New.ConfigKey, add.New.ConfigValue)
    },
    OnUpdate: func(upd types.RecordUpdate[TenantConfig]) error {
        tenant := upd.New.TenantID
        return configService.SetForTenant(tenant, upd.New.ConfigKey, upd.New.ConfigValue)
    },
    OnFinalize: func() error {
        return configService.InvalidateCaches()  // Reload all caches
    },
    Parallelization: ptr(runtime.NumCPU()),
}
```

### Use Case 4: API Endpoint Configuration

**Scenario:** Define API endpoints and permissions in CSV, sync to API gateway.

```go
type APIEndpoint struct {
    Path           string `csv:"path"`
    Method         string `csv:"method"`
    RequiredRole   string `csv:"required_role"`
    RateLimit      int    `csv:"rate_limit"`
    CacheTTL       int    `csv:"cache_ttl"`
}

func endpointKey(e APIEndpoint) string {
    return fmt.Sprintf("%s %s", e.Method, e.Path)
}

params := plan.GenerateParams[APIEndpoint]{
    ExtractKeyFunc: endpointKey,
    LoadRemoteRecords: func() (map[string]APIEndpoint, error) {
        return apiGateway.GetAllEndpoints()
    },
    OnAdd: func(add types.RecordAddition[APIEndpoint]) error {
        return apiGateway.RegisterEndpoint(add.New)
    },
    OnUpdate: func(upd types.RecordUpdate[APIEndpoint]) error {
        return apiGateway.UpdateEndpoint(upd.New)
    },
    OnDelete: func(del types.RecordDeletion[APIEndpoint]) error {
        return apiGateway.DeregisterEndpoint(del.Key)
    },
    OnFinalize: func() error {
        return apiGateway.ReloadConfig()
    },
}
```

## Common Patterns

### Pattern 1: Database Synchronization with Transaction

```go
OnAdd: func(addition types.RecordAddition[UserRecord]) error {
    return db.InsertUser(addition.New)
},
OnUpdate: func(update types.RecordUpdate[UserRecord]) error {
    return db.UpdateUser(update.New)
},
OnDelete: func(deletion types.RecordDeletion[UserRecord]) error {
    return db.DeleteUser(deletion.Key)
},
OnFinalize: func() error {
    return db.Commit()  // Commit all changes in one transaction
},
```

### Pattern 2: Field-Level Change Auditing

Track exactly what changed:

```go
OnUpdate: func(update types.RecordUpdate[UserRecord]) error {
    for _, change := range update.Changes {
        log.Printf(
            "User %s: %s changed from %v to %v",
            update.Key,
            change.Field,
            change.OldValue,
            change.NewValue,
        )
        // Store in audit table
        auditLog.RecordFieldChange(
            update.Key,
            change.Field,
            change.OldValue,
            change.NewValue,
        )
    }
    return db.UpdateUser(update.New)
},
```

### Pattern 3: Conditional Updates (Only Specific Fields)

Only sync certain fields, ignore others:

```go
OnUpdate: func(update types.RecordUpdate[UserRecord]) error {
    // Only update allowed fields
    allowedFields := map[string]bool{
        "points": true,
        "status": true,
    }

    hasAllowedChange := false
    for _, change := range update.Changes {
        if allowedFields[change.Field] {
            hasAllowedChange = true
            break
        }
    }

    if !hasAllowedChange {
        log.Printf("No allowed changes for %s, skipping", update.Key)
        return nil
    }

    return db.UpdateUser(update.New)
},
```

### Pattern 4: Notifications & Events

Send notifications on state changes:

```go
OnAdd: func(addition types.RecordAddition[UserRecord]) error {
    err := db.InsertUser(addition.New)
    if err == nil {
        events.Publish("user.created", addition.New)
        notify.SendSlack(fmt.Sprintf("New user added: %s", addition.New.Name))
    }
    return err
},
OnDelete: func(deletion types.RecordDeletion[UserRecord]) error {
    err := db.DeleteUser(deletion.Key)
    if err == nil {
        events.Publish("user.deleted", deletion.Old)
        notify.SendSlack(fmt.Sprintf("User removed: %s", deletion.Old.Name))
    }
    return err
},
```

### Pattern 5: Retry & Resilience Customization

Handle specific errors with custom retry logic:

```go
OnAdd: func(addition types.RecordAddition[UserRecord]) error {
    err := db.InsertUser(addition.New)

    // Planear auto-retries with exponential backoff
    // But you can add custom handling
    if err != nil && isNetworkError(err) {
        log.Printf("Network error for %s, will be retried automatically", addition.Key)
    }

    return err
},
```

### Pattern 6: Validation with Detailed Feedback

Provide actionable error messages:

```go
ValidateRecord: func(r UserRecord) error {
    errs := []string{}

    if r.Email == "" {
        errs = append(errs, "email required")
    } else if !isValidEmail(r.Email) {
        errs = append(errs, fmt.Sprintf("invalid email format: %s", r.Email))
    }

    if r.Points < 0 {
        errs = append(errs, "points cannot be negative")
    }

    if len(errs) > 0 {
        return fmt.Errorf("validation failed: %s", strings.Join(errs, "; "))
    }

    return nil
},
```

### Pattern 7: Optimize Parallelization

Use all CPU cores for maximum throughput:

```go
numCores := runtime.NumCPU()
params.Parallelization = &numCores
```

For network-bound operations, you might want more workers:

```go
workers := 50  // Handle more concurrent API calls
params.Parallelization = &workers
```

For CPU-bound operations, stick to core count:

```go
workers := runtime.NumCPU()
params.Parallelization = &workers
```

## Data Types

Planear works with any Go struct. Examples:

### Simple Strings

```go
type Tag struct {
    ID   string `csv:"id"`
    Name string `csv:"name"`
}
```

### With Pointers (Optional Fields)

```go
type User struct {
    ID    string  `csv:"id"`
    Name  string  `csv:"name"`
    Phone *string `csv:"phone"`  // Optional: nil if empty in CSV
}
```

### Nested Structs

```go
type Address struct {
    Street string
    City   string
}

type Person struct {
    ID      string  `csv:"id"`
    Name    string  `csv:"name"`
    Address Address `csv:"address"`  // Can be nested
}
```

### Slices

```go
type Group struct {
    ID    string   `csv:"id"`
    Name  string   `csv:"name"`
    Tags  []string `csv:"tags"`  // Parse comma-separated values
}
```

## Error Handling

### Validation Errors

Records failing validation are added to `plan.Ignores` with the error message:

```go
type RecordIgnored[T any] struct {
    Key    string  // The record key
    Record T       // The record that failed
    Reason string  // Validation error message
}
```

### Execution Errors

Failed operations are tracked in the execution report:

```go
type ExecutionReport[T any] struct {
    Success Plan[T]  // Operations that succeeded
    Failure Plan[T]  // Operations that failed
    Ignores []RecordIgnored[T]
}
```

## Testing Your Implementation

### Unit Test Example

```go
package main

import (
    "testing"
    "github.com/algebananazzzzz/planear/pkg/core/plan"
)

func TestUserValidation(t *testing.T) {
    tests := []struct {
        user  UserRecord
        valid bool
    }{
        {
            user:  UserRecord{Email: "alice@example.com", Name: "Alice", Points: 100},
            valid: true,
        },
        {
            user:  UserRecord{Email: "bob@example.com", Name: "Bob", Points: -5},
            valid: false,
        },
    }

    for _, tt := range tests {
        err := ValidateUserRecord(tt.user)
        if (err == nil) != tt.valid {
            t.Errorf("validation failed for %s", tt.user.Email)
        }
    }
}
```

### Integration Test Example

```go
func TestPlanGeneration(t *testing.T) {
    params := plan.GenerateParams[UserRecord]{
        CSVPath:           "testdata",
        OutputFilePath:    "test_plan.json",
        ExtractKeyFunc:    ExtractKey,
        FormatRecordFunc:  FormatRecord,
        FormatKeyFunc:     FormatKey,
        LoadRemoteRecords: func() (map[string]UserRecord, error) {
            return map[string]UserRecord{}, nil
        },
        ValidateRecord: ValidateUserRecord,
    }

    plan, err := plan.Generate(params)
    if err != nil {
        t.Fatalf("plan generation failed: %v", err)
    }

    if len(plan.Additions) == 0 {
        t.Error("expected additions in plan")
    }
}
```

## Tips & Best Practices

1. **Always set parallelization to `runtime.NumCPU()`** for optimal performance on production systems

2. **Use CSV as source of truth** - CSV should always be your authoritative state definition

3. **Validate early** - Catch invalid records during plan generation, not execution

4. **Review plans before execution** - Always review the generated plan (printed to stdout + saved as JSON) before running apply

5. **Idempotent operations** - Make your callbacks safe to run multiple times. The same plan should produce the same result when executed twice

6. **Transaction handling** - Use `OnFinalize` to commit transactions after all operations succeed:
   ```go
   OnFinalize: func() error {
       return db.Commit()
   }
   ```

7. **Error recovery** - Failed operations are automatically retried with exponential backoff. Your callbacks should handle transient failures gracefully

## Next Steps

- Review [README.md](./README.md) for quick start
- See [COMPARISON.md](./COMPARISON.md) for how Planear compares to other tools
- Explore the `/examples` directory in the repository for the complete working example
