docs: 
	-rm docs/*
	cat testdata/go-graphviz.symtab | ./go-binsize-treemap  -csv > docs/go-graphviz.csv
	cat testdata/go-graphviz.symtab | ./go-binsize-treemap > docs/go-graphviz.svg
	cat testdata/go-graphviz.symtab | ./go-binsize-treemap -w 4096 -h 4096 > docs/go-graphviz-4096x4096.svg
	cat testdata/cockroach.symtab | ./go-binsize-treemap -csv > docs/cockroach.csv
	cat testdata/cockroach.symtab | ./go-binsize-treemap > docs/cockroach.svg
	cat testdata/cockroach.symtab | ./go-binsize-treemap -w 4096 -h 4096 > docs/cockroach-4096x4096.svg
	cat testdata/skipper.symtab | ./go-binsize-treemap > docs/skipper.svg
	cat testdata/hugo.symtab | ./go-binsize-treemap -csv > docs/hugo.csv
	cat testdata/hugo.symtab | ./go-binsize-treemap > docs/hugo.svg
	cat testdata/hugo.symtab | ./go-binsize-treemap -w 1024 -h 128 > docs/hugo-1024x128.svg
	cat testdata/hugo.symtab | ./go-binsize-treemap -w 1024 -h 256 > docs/hugo-1024x256.svg
	cat testdata/hugo.symtab | ./go-binsize-treemap -w 1024 -h 512 > docs/hugo-1024x512.svg
	cat testdata/hugo.symtab | ./go-binsize-treemap -w 4096 -h 4096 > docs/hugo-4096x4096.svg
	cat testdata/hugo.symtab | ./go-binsize-treemap -w 16384 -h 16384 > docs/hugo-16384x16384.svg

.PHONY: docs
