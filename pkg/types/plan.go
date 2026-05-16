package types

type Plan[T any] struct {
	Additions []RecordAddition[T] `json:"additions"`
	Updates   []RecordUpdate[T]   `json:"updates"`
	Deletions []RecordDeletion[T] `json:"deletions"`
	Ignores   []RecordIgnored[T]  `json:"ignores"`
	// Layers, if non-nil, dictates apply-time execution order. Each inner
	// slice is a layer; ops in the same layer dispatch in parallel; layer
	// N+1 starts after layer N drains. References ops in Additions /
	// Updates / Deletions by (Kind, Key). Populated by Generate when
	// GenerateParams.DependsOn is set.
	Layers [][]LayerOp `json:"layers,omitempty"`
}

// LayerOp identifies a single operation within a layered execution plan.
// Kind is one of LayerOpAdd / LayerOpUpdate / LayerOpDelete. Key matches
// the operation's Key field within the corresponding Additions / Updates /
// Deletions slice.
type LayerOp struct {
	Kind string `json:"kind"`
	Key  string `json:"key"`
}

type ExecutionReport[T any] struct {
	Success Plan[T] `json:"success"`
	Failure Plan[T] `json:"failure"`
	// Skipped lists ops that were not attempted because an earlier layer
	// failed (layered apply only). Tag intentionally lacks omitempty: callers
	// parsing the report JSON can rely on the field always being present,
	// matching the symmetric treatment of Success and Failure.
	Skipped              Plan[T]            `json:"skipped"`
	Ignores              []RecordIgnored[T] `json:"ignores"`
	FinalizationSuccess  bool               `json:"finalization_success"`
	FinalizationErrorMsg string             `json:"finalization_error_msg,omitempty"`
}

// IsEmpty checks if all lists in the Plan are empty
func (plan *Plan[T]) IsEmpty() bool {
	return len(plan.Additions) == 0 &&
		len(plan.Updates) == 0 &&
		len(plan.Deletions) == 0 &&
		len(plan.Ignores) == 0
}
