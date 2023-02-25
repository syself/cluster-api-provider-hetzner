all: build docs

clean:
	-rm treemap
	-rm *.svg
	-rm coverage.txt
	-rm docs/*.svg

build: clean
	go build ./cmd/treemap

cover:
	go test -cover ./...

docs: 
	cat testdata/gapminder-2007-population-life.csv | ./treemap > docs/gapminder-2007-population-life.svg
	cat testdata/gapminder-2007-population-life.csv | ./treemap -w 1080 -h 1080 > docs/gapminder-2007-population-life-1080x1080.svg
	cat testdata/gapminder-2007-population-life.csv | ./treemap -w 1080 -h 360 > docs/gapminder-2007-population-life-1080x360.svg
	cat testdata/gapminder-2007-population-life.csv | ./treemap -color none > docs/gapminder-2007-population-life-nocolor.svg
	cat testdata/gapminder-2007-population-life.csv | ./treemap -color RedBlu > docs/gapminder-2007-population-life-RedBlu.svg
	cat testdata/gapminder-2007-population-life.csv | ./treemap -impute-heat > docs/gapminder-2007-population-life-impute-heat.svg
	cat testdata/gapminder-2007-population-life.csv | ./treemap -color balanced > docs/gapminder-2007-population-life-balanced.svg
	cat testdata/gapminder-2007-population-life.csv | ./treemap -color RdYlGn > docs/gapminder-2007-population-life-RdYlGn.svg
	cat testdata/long-roots.csv | ./treemap -long-paths -w 1028 -h 256 > docs/long-roots-long-roots.svg
	cat testdata/long-roots.csv | ./treemap -w 1080 -h 256 > docs/long-roots.svg
	cat testdata/gapminder-2007-population-life.csv | ./treemap -long-paths > docs/gapminder-2007-population-life-long-roots.svg
	cat testdata/hugo-binsize.csv | ./treemap > docs/hugo-binsize.svg
	cat testdata/hugo-binsize.csv | ./treemap -color none > docs/hugo-binsize-nocolor.svg
	cat testdata/hugo-binsize.csv | ./treemap -color none -w 4096 -h 4096 -long-paths > docs/hugo-binsize-nocolor-large-long-roots.svg
	cat testdata/hugo-binsize.csv | ./treemap -color none -w 4096 -h 4096 > docs/hugo-binsize-nocolor-large.svg
	#cat testdata/find-src-go-dir.csv | ./treemap -h 4096 -w 4096 > docs/find-src-go-dir.svg

.PHONY: all clean build cover docs
