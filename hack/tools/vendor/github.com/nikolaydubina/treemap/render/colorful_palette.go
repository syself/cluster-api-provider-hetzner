package render

import (
	_ "embed"
	"image/color"
	"log"
	"strconv"
	"strings"

	"github.com/lucasb-eyer/go-colorful"
)

// This table contains the "keypoints" of the colorgradient you want to generate.
// The position of each keypoint has to live in the range [0,1]
// Ths is copied from go-colorful examples!!!
type ColorfulPalette []struct {
	Col colorful.Color
	Pos float64
}

// This is the meat of the gradient computation. It returns a HCL-blend between
// the two colors around `t`.
// Note: It relies heavily on the fact that the gradient keypoints are sorted.
func (gt ColorfulPalette) GetInterpolatedColorFor(t float64) color.Color {
	for i := 0; i < len(gt)-1; i++ {
		c1 := gt[i]
		c2 := gt[i+1]
		if c1.Pos <= t && t <= c2.Pos {
			// We are in between c1 and c2. Go blend them!
			t := (t - c1.Pos) / (c2.Pos - c1.Pos)
			return c1.Col.BlendHcl(c2.Col, t).Clamped()
		}
	}

	// Nothing found? Means we're at (or past) the last gradient keypoint.
	return gt[len(gt)-1].Col
}

//go:embed palettes/ReBu.csv
var paletteReBuCSV string

//go:embed palettes/RdYlGn.csv
var paletteRdYlGnCSV string

func makePaletteFromCSV(csv string) ColorfulPalette {
	rows := strings.Split(csv, "\n")
	palette := make(ColorfulPalette, len(rows))

	for i, row := range rows {
		parts := strings.Split(row, ",")
		if len(parts) != 2 {
			continue
		}

		c, err := colorful.Hex(parts[0])
		if err != nil {
			log.Fatal(err)
		}

		v, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			log.Fatal(err)
		}

		palette[i].Col = c
		palette[i].Pos = v
	}

	return palette
}

func GetPalette(name string) (ColorfulPalette, bool) {
	switch name {
	case "RdBu":
		return makePaletteFromCSV(paletteReBuCSV), true
        case "RdYlGn":
                return makePaletteFromCSV(paletteRdYlGnCSV), true
	default:
		return nil, false
	}
}
