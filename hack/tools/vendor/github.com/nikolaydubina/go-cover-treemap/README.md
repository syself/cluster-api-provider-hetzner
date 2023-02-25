# 🎄 Go cover to Treemap

_Useful when you have large project with lots of files and packages_

> New! Now available at https://go-cover-treemap.io 🎉

```
$ go install github.com/nikolaydubina/go-cover-treemap@latest
$ go test -coverprofile cover.out ./...
$ go-cover-treemap -coverprofile cover.out > out.svg
```

_<p align="center">github.com/gohugoio/hugo</p>_
![example-hugo](docs/hugo.svg)

_<p align="center">..also available in 1080x360</p>_
![example-hugo-small](docs/hugo-1080x360.svg)

_<p align="center">..and even 1080x180</p>_
![example-hugo-small](docs/hugo-1080x180.svg)

_<p align="center">github.com/gin-gonic/gin</p>_
![example-gin](docs/gin.svg)

_<p align="center">github.com/go-chi/chi</p>_
![example-chi](docs/chi.svg)

_<p align="center">github.com/nikolaydubina/treemap</p>_
![example-treemap](docs/go-cover-treemap.svg)

_<p align="center">github.com/nikolaydubina/go-featureprocessing</p>_
![example-go-featureprocessing](docs/go-featureprocessing.svg)

## Disclaimer

In all examples above I run `go test -coverprofile <my-file> ./...`.
I did not do any special setup.
Some projects may require additional steps to properly run tets and generate full coverprofile.
What you see is "lower bound" of coverage for those projects.
All profiles generated on `main` branch of each project in GitHub on 2021-12-07.

## Contributions

Welcomed! Add pretty color palettes! Add interesting examples!

## Reference

* Official Go tool to make HTML from cover profile: https://github.com/golang/go/blob/master/src/cmd/cover/html.go#L97
* Official Go parser of cover profile `golang.org/x/tools/cover`: https://github.com/golang/tools/tree/master/cover
* Go SVG Treemap renderer with treemap: https://github.com/nikolaydubina/treemap

## Appendix A: Statements vs File for Size

You can see that structure and heat changes for `github.com/gohugoio/hugo`.
Subtrees that look bad for files no longer look as bad for statements.
Lots of red boxes for files become very small and unnoticeable.
This can be because they contain non-testable constructs like constants.
It is more accurate to use statments, since heat is percentage of covered statements, and we compute heat by weighting sum by sizes of children.
In short, you are more likely want to use statements for size.

files
![example-hugo-files](docs/hugo-files.svg)

statements
![example-hugo](docs/hugo.svg)

## Appendix B: Long Roots

It is common to have root and first few children to have only one child.
Each takes margin and wastes space.
We can collapse these into longer name, and use that space for visualizing higher depth of boxes.
This is particularly useful for narrow dimensions, which makes feasible useful narrow dimension.

1080x360 with root collapsing
![example-long-root-med-collapsed](docs/hugo-1080x360.svg)

1080x360 without root collapse
![example-long-root-med-no-collapse](docs/hugo-long-root-1080x360.svg)

1080x180 with root collapsing
![example-long-root-med-collapsed](docs/hugo-1080x180.svg)

1080x180 without root collapse
![example-long-root-med-no-collapse](docs/hugo-long-root-1080x180.svg)

## Appendix C: Web UI

Web UI is maintained in dedicated repository: https://github.com/nikolaydubina/go-cover-treemap-web

This is to isolate web (WASM/JS/HTML) needed dependencies, like `syscall/js` from minimal CLI package.

It turns out interactive UI is very helpful. Brower can be utilized as effective input source for:
- changing dimensions of window -> changing dimensions of SVG
- drag and drop file
- slider to increase granularity of treemap

## Appendix D: Only Folders

Projects that have lots of files may benefit from not displaying files 
but only folders.
This is also useful when you want to see impact of immediate files in folder to overall folder heat.
Hierarchical properties of size (sum of sizes of children) and heat (weighted heat of children) are preserved.
Immediate children `.go` files in folder are aggregated into single child `*`.
If there is only one `*` child, then it is skipped
— suggested by [herlon214](https://github.com/herlon214)

files
![example-hugo](docs/hugo.svg)

only folders
![example-hugo-files](docs/hugo-only-folders.svg)

only folders without aggregation (heat property not preserved)
![example-hugo-files](docs/hugo-only-folders-no-aggregation.svg)
