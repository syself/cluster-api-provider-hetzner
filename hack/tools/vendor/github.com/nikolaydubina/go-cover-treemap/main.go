package main

import (
	"flag"
	"fmt"
	"image/color"
	"log"
	"os"

	"golang.org/x/tools/cover"

	"github.com/nikolaydubina/go-cover-treemap/covertreemap"
	"github.com/nikolaydubina/treemap"
	"github.com/nikolaydubina/treemap/render"
)

const doc string = `
Generate heat treemaps for cover Go cover profile.

Example:

$ go test -coverprofile cover.out ./...
$ go-cover-heatmap -coverprofile cover.out > out.svg

Command options:
`

var grey = color.RGBA{128, 128, 128, 255}

func main() {
	var (
		coverprofile    string
		w               float64
		h               float64
		marginBox       float64
		paddingBox      float64
		padding         float64
		imputeHeat      bool
		countStatements bool
		collapseRoot    bool
		onlyFolders     bool
	)

	flag.Usage = func() {
		fmt.Fprint(flag.CommandLine.Output(), doc)
		flag.PrintDefaults()
	}
	flag.StringVar(&coverprofile, "coverprofile", "", "filename of input coverprofile (e.g. cover.out)")
	flag.Float64Var(&w, "w", 1028, "width of output")
	flag.Float64Var(&h, "h", 640, "height of output")
	flag.Float64Var(&marginBox, "margin-box", 4, "margin between boxes")
	flag.Float64Var(&paddingBox, "padding-box", 4, "padding between box border and content")
	flag.Float64Var(&padding, "padding", 16, "padding around root content")
	flag.BoolVar(&imputeHeat, "impute-heat", true, "impute heat for parents(weighted sum) and leafs(0.5)")
	flag.BoolVar(&countStatements, "statements", true, "count statemtents in files for size of files, when false then each file is size 1")
	flag.BoolVar(&collapseRoot, "collapse-root", true, "if true then will collapse roots that have one child")
	flag.BoolVar(&onlyFolders, "only-folders", false, "if true then do not display files")
	flag.Parse()

	var err error
	var profiles []*cover.Profile
	if coverprofile != "" {
		profiles, err = cover.ParseProfiles(coverprofile)
	} else {
		profiles, err = cover.ParseProfilesFromReader(os.Stdin)
	}
	if err != nil {
		log.Fatal(err)
	}

	treemapBuilder := covertreemap.NewCoverageTreemapBuilder(countStatements)
	tree, err := treemapBuilder.CoverageTreemapFromProfiles(profiles)
	if err != nil {
		log.Fatal(err)
	}

	sizeImputer := treemap.SumSizeImputer{EmptyLeafSize: 1}
	sizeImputer.ImputeSize(*tree)
	treemap.SetNamesFromPaths(tree)

	weightImputer := treemap.WeightedHeatImputer{EmptyLeafHeat: 1}
	weightImputer.ImputeHeat(*tree)

	if collapseRoot {
		treemap.CollapseLongPaths(tree)
	}

	if imputeHeat {
		heatImputer := treemap.WeightedHeatImputer{EmptyLeafHeat: 0.5}
		heatImputer.ImputeHeat(*tree)
	}

	if onlyFolders {
		if !imputeHeat {
			log.Fatal("impute-heat has to be true")
		}
		covertreemap.AggregateGoFilesTreemapFilter(tree)
		covertreemap.RemoveGoFilesTreemapFilter(tree)
		covertreemap.CollapseRootsWithoutNameTreemapFilter(tree)
	}

	palette, ok := render.GetPalette("RdYlGn")
	if !ok {
		log.Fatalf("can not get palette")
	}
	uiBuilder := render.UITreeMapBuilder{
		Colorer:     render.HeatColorer{Palette: palette},
		BorderColor: grey,
	}
	spec := uiBuilder.NewUITreeMap(*tree, w, h, marginBox, paddingBox, padding)
	renderer := render.SVGRenderer{}

	os.Stdout.Write(renderer.Render(spec, w, h))
}
