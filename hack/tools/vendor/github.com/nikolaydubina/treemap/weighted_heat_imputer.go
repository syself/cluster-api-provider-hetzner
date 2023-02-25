package treemap

// WeightedHeatImputer will make color of parent to weighted sum of colors of its children.
type WeightedHeatImputer struct {
	EmptyLeafHeat float64
}

func (s WeightedHeatImputer) ImputeHeat(t Tree) {
	s.ImputeHeatNode(t, t.Root)
}

func (s WeightedHeatImputer) ImputeHeatNode(t Tree, node string) {
	var heats []float64
	var sizes []float64

	for _, child := range t.To[node] {
		s.ImputeHeatNode(t, child)

		if t.Nodes[child].HasHeat {
			sizes = append(sizes, t.Nodes[child].Size)
			heats = append(heats, t.Nodes[child].Heat)
		}
	}

	if n, ok := t.Nodes[node]; !ok || !n.HasHeat {
		v := s.EmptyLeafHeat
		if len(t.To[node]) > 0 {
			var totalSize float64
			for _, childSize := range sizes {
				totalSize += childSize
			}

			v = 0.0
			for i := range sizes {
				v += heats[i] * sizes[i]
			}
			v /= totalSize
		}

		t.Nodes[node] = Node{
			Path:    t.Nodes[node].Path,
			Name:    t.Nodes[node].Name,
			Size:    t.Nodes[node].Size,
			Heat:    v,
			HasHeat: true,
		}
	}
}
