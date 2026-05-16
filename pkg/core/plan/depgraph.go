package plan

import (
	"github.com/algebananazzzzz/planear/pkg/internal/dag"
	"github.com/algebananazzzzz/planear/pkg/types"
)

// ComputeLayers builds the dependency DAG for a Plan and returns its
// topological layering.
//
// extractKey(record) returns the record's identity (must match the Key field
// of the corresponding RecordAddition / RecordUpdate / RecordDeletion).
// dependsOn(record) returns the keys the record references. Keys not present
// in the plan are treated as external (impose no in-plan ordering).
//
// Deletion inversion: a deletion node is scheduled AFTER every other op
// whose effective state references the deleted key. For Additions, the NEW
// state is checked. For Updates, BOTH the NEW state AND the OLD state are
// checked (the old reference must be cleared by the update before the
// deletion can drop the referenced row).
//
// Returns an error of the form "cycle detected: ..." if the dependency graph
// contains a cycle.
func ComputeLayers[T any](
	p types.Plan[T],
	extractKey func(T) string,
	dependsOn func(T) []string,
) ([][]types.LayerOp, error) {
	_ = extractKey // kept in signature to document the contract; Key fields on
	// RecordAddition / RecordUpdate / RecordDeletion are already populated.

	// Build the op list with kind/key plus the resolved dependency sets.
	type opNode struct {
		layerOp types.LayerOp
		nodeID  string
		key     string
		newDeps []string
		oldDeps []string
	}

	idOf := func(kind, key string) string { return kind + ":" + key }

	ops := make([]opNode, 0, len(p.Additions)+len(p.Updates)+len(p.Deletions))
	for _, a := range p.Additions {
		ops = append(ops, opNode{
			layerOp: types.LayerOp{Kind: types.LayerOpAdd, Key: a.Key},
			nodeID:  idOf(types.LayerOpAdd, a.Key),
			key:     a.Key,
			newDeps: dependsOn(a.New),
		})
	}
	for _, u := range p.Updates {
		ops = append(ops, opNode{
			layerOp: types.LayerOp{Kind: types.LayerOpUpdate, Key: u.Key},
			nodeID:  idOf(types.LayerOpUpdate, u.Key),
			key:     u.Key,
			newDeps: dependsOn(u.New),
			oldDeps: dependsOn(u.Old),
		})
	}
	for _, d := range p.Deletions {
		ops = append(ops, opNode{
			layerOp: types.LayerOp{Kind: types.LayerOpDelete, Key: d.Key},
			nodeID:  idOf(types.LayerOpDelete, d.Key),
			key:     d.Key,
			oldDeps: dependsOn(d.Old),
		})
	}

	// Index ops by their key for fast lookup during edge construction.
	// A given key may have at most one add or one update (not both — Plan
	// invariant). A deletion uses the same key namespace.
	addOrUpdateByKey := make(map[string]*opNode, len(ops))
	for i := range ops {
		o := &ops[i]
		if o.layerOp.Kind == types.LayerOpAdd || o.layerOp.Kind == types.LayerOpUpdate {
			addOrUpdateByKey[o.key] = o
		}
	}

	// Assemble node list and edge map for BuildLayers.
	nodes := make([]string, 0, len(ops))
	nodeToOp := make(map[string]types.LayerOp, len(ops))
	for _, o := range ops {
		nodes = append(nodes, o.nodeID)
		nodeToOp[o.nodeID] = o.layerOp
	}
	edges := make(map[string][]string, len(ops))

	for i := range ops {
		o := &ops[i]
		switch o.layerOp.Kind {
		case types.LayerOpAdd, types.LayerOpUpdate:
			// O depends on the add/update op of every key it references.
			for _, depKey := range o.newDeps {
				if dep, ok := addOrUpdateByKey[depKey]; ok {
					edges[o.nodeID] = append(edges[o.nodeID], dep.nodeID)
				}
			}
		}
	}

	// Deletion inversion.
	for i := range ops {
		o := &ops[i]
		if o.layerOp.Kind != types.LayerOpDelete {
			continue
		}
		delKey := o.key
		for j := range ops {
			other := &ops[j]
			if other.nodeID == o.nodeID {
				continue
			}
			refs := false
			switch other.layerOp.Kind {
			case types.LayerOpAdd:
				refs = containsString(other.newDeps, delKey)
			case types.LayerOpUpdate:
				refs = containsString(other.newDeps, delKey) || containsString(other.oldDeps, delKey)
			}
			if refs {
				edges[o.nodeID] = append(edges[o.nodeID], other.nodeID)
			}
		}
	}

	layeredIDs, err := dag.BuildLayers(nodes, edges)
	if err != nil {
		return nil, err
	}

	if len(layeredIDs) == 0 {
		return nil, nil
	}
	result := make([][]types.LayerOp, len(layeredIDs))
	for i, layer := range layeredIDs {
		result[i] = make([]types.LayerOp, len(layer))
		for j, id := range layer {
			result[i][j] = nodeToOp[id]
		}
	}
	return result, nil
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
