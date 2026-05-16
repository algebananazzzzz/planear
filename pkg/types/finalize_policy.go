package types

// FinalizeOn controls when ExecuteOperations invokes the OnFinalize callback.
// Zero value = FinalizeAlways (preserves pre-Layers behavior).
type FinalizeOn int

const (
	// FinalizeAlways runs OnFinalize regardless of failures or skipped ops.
	// Default; preserves backward compatibility.
	FinalizeAlways FinalizeOn = iota
	// FinalizeOnSuccess runs OnFinalize only when no op failed and no op was
	// skipped (i.e. the plan ran to completion).
	FinalizeOnSuccess
	// FinalizeOnAnySuccess runs OnFinalize when at least one op succeeded;
	// skips it only on zero progress. Recommended for new consumers.
	FinalizeOnAnySuccess
)
