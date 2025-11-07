package types

type Plan[T any] struct {
	Additions []RecordAddition[T] `json:"additions"`
	Updates   []RecordUpdate[T]   `json:"updates"`
	Deletions []RecordDeletion[T] `json:"deletions"`
	Ignores   []RecordIgnored[T]  `json:"ignores"`
}

type ExecutionReport[T any] struct {
	Success              Plan[T]            `json:"success"`
	Failure              Plan[T]            `json:"failure"`
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
