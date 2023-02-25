# 🍬 Pretty Treemaps

> _Looking to run this for Go coverage? Check https://github.com/nikolaydubina/go-cover-treemap_

[![Go Reference](https://pkg.go.dev/badge/github.com/nikolaydubina/treemap.svg)](https://pkg.go.dev/github.com/nikolaydubina/treemap)
[![codecov](https://codecov.io/gh/nikolaydubina/treemap/branch/main/graph/badge.svg?token=h3S894POFO)](https://codecov.io/gh/nikolaydubina/treemap)
[![Go Report Card](https://goreportcard.com/badge/github.com/nikolaydubina/treemap)](https://goreportcard.com/report/github.com/nikolaydubina/treemap)

```bash
$ go install github.com/nikolaydubina/treemap/cmd/treemap@latest 
$ echo '
Africa/Algeria,33333216,72
Africa/Angola,12420476,42
Africa/Benin,8078314,56
...
' | treemap > out.svg
```
![example](./docs/gapminder-2007-population-life.svg)

Adjusting size
```bash
$ ... | treemap -w 1080 -h 360 > out.svg
```
![example-narrow](./docs/gapminder-2007-population-life-1080x360.svg)

```bash
$ ... | treemap -w 1080 -h 1080 > out.svg
```
![example-square](./docs/gapminder-2007-population-life-1080x1080.svg)

Imputing heat
```bash
$ ... | treemap -impute-heat > out.svg
```
![example-narrow](./docs/gapminder-2007-population-life-impute-heat.svg)

Different colorscheme
```bash
$ ... | treemap -color RdYlGn > out.svg
```
![example-RdYlGn](./docs/gapminder-2007-population-life-RdYlGn.svg)


Tree-Hue coloring when there is no heat
```
$ ... | treemap -color balanced > out.svg
```
![example-balanced](./docs/gapminder-2007-population-life-balanced.svg)

Without color
```bash
$ ... | treemap -color none > out.svg
```
![example-no-color](./docs/gapminder-2007-population-life-nocolor.svg)

## Format

Size and heat is optional.

```
</ delimitered path>,<size>,<heat>
```

## Algorithms

* `Squarified` algorithm for treemap layout problem. This is very common algorithm used in Plotly and most of visualization packages. _"Squarified Treemaps", Mark Bruls, Kees Huizing, and Jarke J. van Wijk, 2000_
* `Tree-Hue Color` algorithm for generating colors for nodes in treemap. The idea is to represent hierarchical structure by recursively painting similar hue to subtrees. _Nikolay Dubina, 2021_


## Contributions

Welcomed!

## References

* Plotly treemaps: https://plotly.com/python/treemaps/
* go-colorful: https://github.com/lucasb-eyer/go-colorful
* D3 treemap is using Squerified: https://github.com/d3/d3-hierarchy
* Interactive treemap: https://github.com/vasturiano/treemap-chart
* Squerified in Rust: https://github.com/bacongobbler/treemap-rs
* Squerified in JavaScript: https://github.com/clementbat/treemap
* Squerified in Python: https://github.com/laserson/squarify
* Treemap Go tool: https://github.com/willpoint/treemap
* Plotly color scales: https://plotly.com/python/builtin-colorscales
* Plotly color scales source: https://github.com/plotly/plotly.py/blob/master/packages/python/plotly/_plotly_utils/colors/colorbrewer.py
* Colorbrewer project, that is used in Plotly: http://colorbrewer2.org

## Appendix A: Long Roots

When roots have one child multiple times it takes extra vertical space, which is very useful for narrow final dimensions.

![example-long-roots](./docs/long-roots-long-roots.svg)

Can collapse them into one node
![example-long-roots-collapse](./docs/long-roots.svg)

Long roots without collapsing somewhere deep inside

![](./docs/hugo-binsize-nocolor-large-long-roots.svg)

Long roots with collapsing somewhere deep inside

![](./docs/hugo-binsize-nocolor-large.svg)

## Appendix B: Less Illustrative Examples

Large dimensions and large tree (e.g. `github.com/golang/go`)
```bash
$ ... | treemap -w 4096 -h 4096 > out.svg
```
![example-large](./docs/find-src-go-dir.svg)
