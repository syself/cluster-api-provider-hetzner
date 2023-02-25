package layout

import (
	"math"
	"sort"
)

type Box struct {
	X float64
	Y float64
	W float64
	H float64
}

var NilBox Box = Box{}

type wrappedArea struct {
	i    int
	area float64
}

// Squarify partitions box into parts by using Squarify algorithm.
// As described in "Squarified Treemaps", Mark Bruls, Kees Huizing, and Jarke J. van Wijk., 2000
// This function does sanity checks and hardening so that algorithm can work in the wild.
// Returns boxes in same order as areas.
// Zero areas will have zero-value box.
func Squarify(box Box, areas []float64) []Box {
	// normalize and sort from highest to lowest
	sortedAreas := make([]wrappedArea, len(areas))
	for i, s := range normalizeAreas(areas, (box.W * box.H)) {
		sortedAreas[i] = wrappedArea{i: i, area: s}
	}
	sort.Slice(sortedAreas, func(i, j int) bool { return sortedAreas[i].area > sortedAreas[j].area })

	// take non zero areas only, zero areas are to the right
	cleanAreas := make([]float64, 0, len(areas))
	for _, v := range sortedAreas {
		if v.area > 0 {
			cleanAreas = append(cleanAreas, v.area)
		}
	}

	// squarify
	layout := squarifyBoxLayout{
		boxes:     nil,
		freeSpace: box,
	}
	layout.squarify(cleanAreas, nil, math.Min(layout.freeSpace.W, layout.freeSpace.H))

	boxes := layout.boxes
	cutoffOverflows(box, layout.boxes)

	// restore ordering
	res := make([]Box, len(areas))
	for i, wr := range sortedAreas {
		if i < len(cleanAreas) && i < len(boxes) {
			// this area has some value
			res[wr.i] = boxes[i]
		} else {
			// zero value, this area was zero
			res[wr.i] = Box{}
		}
	}

	return res
}

func normalizeAreas(areas []float64, target float64) []float64 {
	var total float64
	for _, s := range areas {
		total += s
	}
	if total == target {
		return areas
	}
	n := make([]float64, len(areas))
	copy(n, areas)
	for i, s := range n {
		n[i] = target * s / total
	}
	return n
}

// squarifyBoxLayout defines how to partition BoundingBox into boxes
type squarifyBoxLayout struct {
	boxes     []Box // fixed boxes that have been positioned and sized already
	freeSpace Box   // free space that is left. will be used to fill out with new boxes
}

// squarify expects normalized areas that add up to free space.
// areas should not be zero.
func (l *squarifyBoxLayout) squarify(unassignedAreas []float64, stackAreas []float64, w float64) {
	if len(unassignedAreas) == 0 {
		l.stackBoxes(stackAreas)
		return
	}

	if len(stackAreas) == 0 {
		l.squarify(unassignedAreas[1:], []float64{unassignedAreas[0]}, w)
		return
	}

	c := unassignedAreas[0]
	if stackc := append(stackAreas, c); highestAspectRatio(stackAreas, w) > highestAspectRatio(stackc, w) {
		// aspect ratio improves, add it to current stack
		l.squarify(unassignedAreas[1:], stackc, w)
	} else {
		// aspect ratio does not improve
		l.stackBoxes(stackAreas)
		l.squarify(unassignedAreas, nil, math.Min(l.freeSpace.W, l.freeSpace.H))
	}
}

// stackBoxes makes new boxes accordingly to areas and fix them into freeSpacelayout within bounding box
func (l *squarifyBoxLayout) stackBoxes(stackAreas []float64) {
	if l.freeSpace.W < l.freeSpace.H {
		l.stackBoxesHorizontal(stackAreas)
	} else {
		l.stackBoxesVertical(stackAreas)
	}
}

// stackBoxesVertical takes vertical chunk of free space of bounding box and partitiones it into areas
func (l *squarifyBoxLayout) stackBoxesVertical(areas []float64) {
	if len(areas) == 0 {
		return
	}

	stackArea := 0.0
	for _, s := range areas {
		stackArea += s
	}
	if stackArea == 0 {
		return
	}

	totalArea := l.freeSpace.W * l.freeSpace.H
	if totalArea == 0 {
		return
	}

	// stack
	offset := l.freeSpace.Y
	for _, s := range areas {
		h := l.freeSpace.H * s / stackArea
		b := Box{
			X: l.freeSpace.X,
			W: l.freeSpace.W * stackArea / totalArea,
			Y: offset,
			H: h,
		}
		offset += h
		l.boxes = append(l.boxes, b)
	}

	// shrink free space
	l.freeSpace = Box{
		X: l.freeSpace.X + (l.freeSpace.W * stackArea / totalArea),
		W: l.freeSpace.W * (1 - (stackArea / totalArea)),
		Y: l.freeSpace.Y,
		H: l.freeSpace.H,
	}
}

// stackBoxesHorizontal takes horizontal chunk of free space of bounding box and partitiones it into areas
func (l *squarifyBoxLayout) stackBoxesHorizontal(areas []float64) {
	if len(areas) == 0 {
		return
	}

	stackArea := 0.0
	for _, s := range areas {
		stackArea += s
	}
	if stackArea == 0 {
		return
	}

	totalArea := l.freeSpace.W * l.freeSpace.H
	if totalArea == 0 {
		return
	}

	// stack
	offset := l.freeSpace.X
	for _, s := range areas {
		w := l.freeSpace.W * s / stackArea
		b := Box{
			X: offset,
			W: w,
			Y: l.freeSpace.Y,
			H: l.freeSpace.H * stackArea / totalArea,
		}
		offset += w
		l.boxes = append(l.boxes, b)
	}

	// shrink free space
	l.freeSpace = Box{
		X: l.freeSpace.X,
		W: l.freeSpace.W,
		Y: l.freeSpace.Y + (l.freeSpace.H * stackArea / totalArea),
		H: l.freeSpace.H * (1 - (stackArea / totalArea)),
	}
}

// highestAspectRatio of a list of rectangles's areas, given the length of the side along which they are to be laid out
func highestAspectRatio(areas []float64, w float64) float64 {
	var minArea, maxArea, totalArea float64
	for i, s := range areas {
		totalArea += s
		if i == 0 || s < minArea {
			minArea = s
		}
		if i == 0 || s > maxArea {
			maxArea = s
		}
	}

	v1 := w * w * maxArea / (totalArea * totalArea)
	v2 := totalArea * totalArea / (w * w * minArea)

	return math.Max(v1, v2)
}

// cutoffOverflows will set boxes that overflow to fit into bounding box.
// This is useful for numerical stability on the borders.
func cutoffOverflows(boundingBox Box, boxes []Box) {
	maxX := boundingBox.X + boundingBox.W
	maxY := boundingBox.Y + boundingBox.H

	for i, b := range boxes {
		if delta := (b.X + b.W) - maxX; delta > 0 {
			boxes[i].W -= delta
		}
		if delta := (b.Y + b.H) - maxY; delta > 0 {
			boxes[i].H -= delta
		}
	}
}
