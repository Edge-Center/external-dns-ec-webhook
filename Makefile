VERSION := $(shell git describe --tags --always --dirty)

.PHONY: build
build: ## build the binary
	CGO_ENABLED=0 go build -ldflags "-X main.Version=$(VERSION)" -o external-dns-ec-webhook .

.PHONY: run
run:build ## run the binary
	./external-dns-ec-webhook