package types

// LayerOpKind identifies which class of operation a LayerOp represents.
// The underlying string type is preserved so JSON encoding remains
// "add"/"update"/"delete", matching pre-typed-enum plan files on disk.
type LayerOpKind string

const (
	LayerOpAdd    LayerOpKind = "add"
	LayerOpUpdate LayerOpKind = "update"
	LayerOpDelete LayerOpKind = "delete"
)
