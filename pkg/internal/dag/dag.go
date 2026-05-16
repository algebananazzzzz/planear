// Package dag implements layered topological sorting with cycle detection.
// It is internal to planear and not part of any public API.
package dag

import (
	"fmt"
	"sort"
	"strings"
)

// BuildLayers performs a layered topological sort over the given nodes.
//
// edges[v] returns the keys that v depends on. A dependency on a key not in
// nodes is treated as already satisfied (e.g. references to remote-only rows
// that impose no in-plan ordering).
//
// Within a layer, nodes are returned in lexicographic order so callers get
// reproducible output across runs.
//
// Returns an error of the form "cycle detected: A -> B -> ... -> A" if the
// graph contains a cycle.
func BuildLayers(nodes []string, edges map[string][]string) ([][]string, error) {
	if len(nodes) == 0 {
		return nil, nil
	}

	nodeSet := make(map[string]struct{}, len(nodes))
	for _, n := range nodes {
		nodeSet[n] = struct{}{}
	}

	indegree := make(map[string]int, len(nodes))
	reverse := make(map[string][]string, len(nodes))
	for _, v := range nodes {
		indegree[v] = 0
	}
	for _, v := range nodes {
		for _, dep := range edges[v] {
			if _, ok := nodeSet[dep]; !ok {
				continue
			}
			indegree[v]++
			reverse[dep] = append(reverse[dep], v)
		}
	}

	var layers [][]string
	remaining := len(nodes)

	for remaining > 0 {
		var layer []string
		for _, v := range nodes {
			if indegree[v] == 0 {
				layer = append(layer, v)
			}
		}
		if len(layer) == 0 {
			return nil, formatCycleError(nodes, indegree, edges, nodeSet)
		}
		sort.Strings(layer)
		layers = append(layers, layer)
		for _, v := range layer {
			indegree[v] = -1
			for _, downstream := range reverse[v] {
				if indegree[downstream] > 0 {
					indegree[downstream]--
				}
			}
		}
		remaining -= len(layer)
	}

	return layers, nil
}

func formatCycleError(nodes []string, indegree map[string]int, edges map[string][]string, nodeSet map[string]struct{}) error {
	var start string
	for _, n := range nodes {
		if indegree[n] > 0 {
			start = n
			break
		}
	}

	visited := map[string]int{}
	var path []string
	current := start
	for {
		if pos, seen := visited[current]; seen {
			path = append(path, current)
			cycle := path[pos:]
			return fmt.Errorf("cycle detected: %s", strings.Join(cycle, " -> "))
		}
		visited[current] = len(path)
		path = append(path, current)

		var next string
		for _, dep := range edges[current] {
			if _, in := nodeSet[dep]; !in {
				continue
			}
			if indegree[dep] > 0 {
				next = dep
				break
			}
		}
		if next == "" {
			return fmt.Errorf("cycle detected (path truncated): %s", strings.Join(path, " -> "))
		}
		current = next
	}
}
