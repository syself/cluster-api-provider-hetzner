docs: 
	cat testdata/treemap.cover | ./go-cover-treemap > docs/go-cover-treemap-stdin.svg
	./go-cover-treemap -coverprofile testdata/treemap.cover > docs/go-cover-treemap.svg
	./go-cover-treemap -coverprofile testdata/go-featureprocessing.cover > docs/go-featureprocessing.svg
	./go-cover-treemap -coverprofile testdata/gin.cover > docs/gin.svg
	./go-cover-treemap -coverprofile testdata/chi.cover > docs/chi.svg
	./go-cover-treemap -coverprofile testdata/hugo.cover > docs/hugo.svg
	./go-cover-treemap -coverprofile testdata/hugo.cover -w 1080 -h 360 > docs/hugo-1080x360.svg
	./go-cover-treemap -coverprofile testdata/hugo.cover -w 1080 -h 180 > docs/hugo-1080x180.svg
	./go-cover-treemap -coverprofile testdata/hugo.cover -statements=false > docs/hugo-files.svg
	./go-cover-treemap -coverprofile testdata/hugo.cover -collapse-root=false > docs/hugo-long-root.svg
	./go-cover-treemap -coverprofile testdata/hugo.cover -collapse-root=false -w 1080 -h 360 > docs/hugo-long-root-1080x360.svg
	./go-cover-treemap -coverprofile testdata/hugo.cover -collapse-root=false -w 1080 -h 180 > docs/hugo-long-root-1080x180.svg
	./go-cover-treemap -coverprofile testdata/hugo.cover -only-folders > docs/hugo-only-folders.svg

.PHONY: docs
