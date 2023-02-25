package render

import (
	"image/color"
	"math"

	"github.com/lucasb-eyer/go-colorful"
	"github.com/nikolaydubina/treemap"
)

// TreeHueColorer this algorithm will split Hue in NCL ranges such that deeper nodes have more specific hue.
// The advantage of this coloring is that nodes in that belong topologically close will have similar hue.
// Supposed to be run once on tree due to memoization.
// The challenge that not all HCL values are valid colors. Which is why we have to sample and look for value within range.
// For very deep trees, that require precise colors colors closer to leaves will get mixed due to sampling.
type TreeHueColorer struct {
	Hues   map[string]float64 // memoized hues
	C      float64            // will be in all colors
	L      float64            // will be in all colors
	Offset float64            // 0 ~ 360 hue offset in HCL for tree
	DeltaH float64            // tolerance for approximate color
	DeltaC float64            // tolerance for approximate color
	DeltaL float64            // tolerance for approximate color
}

func (s TreeHueColorer) ColorBox(tree treemap.Tree, node string) color.Color {
	if len(s.Hues) == 0 {
		for k, v := range TreeHues(tree, s.Offset) {
			s.Hues[k] = v
		}
	}

	// some of HCL is not valid. using generator from go-colorful package to get one colour
	f := func(l, a, b float64) bool {
		// target
		th, tc, tl := s.Hues[node], s.C, s.L
		// current
		h, c, l := colorful.LabToHcl(l, a, b)
		// withing range
		return (math.Abs(h-th) < s.DeltaH) && (math.Abs(c-tc) < s.DeltaC) && (math.Abs(l-tl) < s.DeltaL)
	}
	palette, err := colorful.SoftPaletteEx(1, colorful.SoftPaletteSettings{CheckColor: f, Iterations: 500, ManySamples: true})
	if err != nil {
		// white
		return colorful.Hcl(0, 0, 1)
	}

	return palette[0]
}

func (s TreeHueColorer) ColorText(tree treemap.Tree, node string) color.Color {
	boxColor := s.ColorBox(tree, node).(colorful.Color)
	_, _, l := boxColor.Hcl()
	switch {
	case l > 0.5:
		return DarkTextColor
	default:
		return LightTextColor
	}
}

func TreeHues(tree treemap.Tree, offset float64) map[string]float64 {
	ranges := map[string][2]float64{tree.Root: {offset, 360 + offset}}

	que := []string{tree.Root}
	var q string
	for len(que) > 0 {
		q, que = que[0], que[1:]
		children := tree.To[q]
		que = append(que, children...)

		if len(children) == 0 {
			continue
		}

		if len(children) == 1 {
			// same color as parent
			ranges[children[0]] = ranges[q]
			continue
		}

		if len(children) > 1 {
			// for N children we allocating N parts of parent's range
			minH, maxH := ranges[q][0], ranges[q][1]

			split := minH
			w := math.Abs(maxH-minH) / float64(len(children))
			for i, child := range children {
				if i == (len(children) - 1) {
					ranges[child] = [2]float64{split, maxH}
					continue
				}
				ranges[child] = [2]float64{split, split + w}
				split += w
			}
		}
	}

	// final normalized hue
	hues := map[string]float64{}

	// parts of edges
	for node := range tree.To {
		minH, maxH := ranges[node][0], ranges[node][1]
		hues[node] = math.Mod(((minH + maxH) / 2), 360)
	}

	// leaf nodes
	for node := range tree.Nodes {
		minH, maxH := ranges[node][0], ranges[node][1]
		hues[node] = math.Mod(((minH + maxH) / 2), 360)
	}

	return hues
}
