package types

type FieldChange struct {
	Field    string `json:"field"`
	OldValue any    `json:"old_value"`
	NewValue any    `json:"new_value"`
}

// RecordAddition represents a new record that will be added.
type RecordAddition[T any] struct {
	Key string `json:"key"`
	New T      `json:"new"`
}

// RecordUpdate represents a record that will be updated.
type RecordUpdate[T any] struct {
	Key     string        `json:"key"`
	Changes []FieldChange `json:"changes"`
	Old     T             `json:"old"`
	New     T             `json:"new"`
}

// RecordDeletion represents a record that will be removed.
type RecordDeletion[T any] struct {
	Key string `json:"key"`
	Old T      `json:"old"`
}

// RecordIgnored represents a record that was skipped for some reason.
type RecordIgnored[T any] struct {
	Key    string `json:"key"`
	Record T      `json:"record"`
	Reason string `json:"reason"`
}
