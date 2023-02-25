package covertreemap

import (
	"strings"

	"github.com/nikolaydubina/treemap"
)

// RemoveGoFilesTreemapFilter removes .go files from Treemap.
// Size and heat of parents have to be already imputed.
func RemoveGoFilesTreemapFilter(tree *treemap.Tree) {
	for path := range tree.Nodes {
		if strings.HasSuffix(path, ".go") {
			delete(tree.Nodes, path)
		}
	}

	for parent, children := range tree.To {
		childrenNew := make([]string, 0, len(children))

		for _, child := range children {
			if !strings.HasSuffix(child, ".go") {
				childrenNew = append(childrenNew, child)
			}
		}

		tree.To[parent] = childrenNew
	}
}

// AggregateGoFilesTreemapFilter aggregates .go files from Treemap
// in each parent into single node `*`.
func AggregateGoFilesTreemapFilter(tree *treemap.Tree) {
	// store coverage statement per aggregated node
	aggcov := make(map[string]float64)

	for path, node := range tree.Nodes {
		if !strings.HasSuffix(path, ".go") {
			continue
		}

		parent := parent(path)
		aggPath := parent + "/" + "*"

		// check if has new node edge
		hasNewNode := false
		for _, to := range tree.To[parent] {
			if to == aggPath {
				hasNewNode = true
				break
			}
		}
		if !hasNewNode {
			tree.To[parent] = append(tree.To[parent], aggPath)
		}

		// update aggregated node values
		if _, ok := tree.Nodes[aggPath]; !ok {
			tree.Nodes[aggPath] = treemap.Node{
				Path:    aggPath,
				Name:    "*",
				Size:    0,
				Heat:    0,
				HasHeat: true,
			}
		}
		aggNode := tree.Nodes[aggPath]
		aggNode.Size += node.Size
		tree.Nodes[aggPath] = aggNode

		aggcov[aggPath] += node.Size * node.Heat
	}

	// set heat
	for aggPath, cov := range aggcov {
		aggNode := tree.Nodes[aggPath]
		aggNode.Heat = cov / aggNode.Size
		tree.Nodes[aggPath] = aggNode
	}
}

// CollapseRootsWithoutNameTreemapFilter collapses roots with single child without updating name of parents.
func CollapseRootsWithoutNameTreemapFilter(tree *treemap.Tree) {
	for path := range tree.Nodes {
		parent := parent(path)
		if len(tree.To[parent]) == 1 {
			tree.To[parent] = nil
			delete(tree.Nodes, path)
		}
	}
}

func parent(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return ""
	}
	return strings.Join(parts[:len(parts)-1], "/")
}
