GO := $(shell which go)

test:
	${GO} test -race ./...

cover:
	${GO} test -race -coverprofile=coverage.out ./...
	${GO} tool cover -func coverage.out

fmt:
	${GO} fmt ./...

.PHONY: test cover fmt