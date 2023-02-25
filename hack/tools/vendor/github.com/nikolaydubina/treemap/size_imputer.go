package treemap

import "strings"

// SumSizeImputer will set sum of children into empty parents and fill children with contant.
type SumSizeImputer struct {
	EmptyLeafSize float64
}

func (s SumSizeImputer) ImputeSize(t Tree) {
	s.ImputeSizeNode(t, t.Root)
}

func (s SumSizeImputer) ImputeSizeNode(t Tree, node string) {
	var sum float64
	for _, child := range t.To[node] {
		s.ImputeSizeNode(t, child)
		sum += t.Nodes[child].Size
	}

	if n, ok := t.Nodes[node]; !ok || n.Size == 0 {
		v := s.EmptyLeafSize
		if len(t.To[node]) > 0 {
			v = sum
		}

		var name string
		if parts := strings.Split(node, "/"); len(parts) > 0 {
			name = parts[len(parts)-1]
		}

		t.Nodes[node] = Node{
			Path: node,
			Name: name,
			Size: v,
		}
	}
}
