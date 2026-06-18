GO = go
SCDOC = scdoc
LDFLAGS = "-s -w"

pkgs = ./...

.PHONY: all
all:
	$(GO) build ./cmd/shoelaces

.PHONY: dev
dev:
	$(GO) run ./cmd/shoelaces --config dev/shoelaces.yaml

.PHONY: lint
lint:
	golangci-lint run -v

.PHONY: fmt
fmt:
	$(GO) fmt ./...

.PHONY: clean
clean:
	rm -f shoelaces docs/shoelaces.8

shoelaces.8:
	$(SCDOC) < docs/shoelaces.8.scd > docs/shoelaces.8

.PHONY: docs
docs: shoelaces.8

.PHONY: unit
unit: fmt
	$(GO) test -v -count=1 $(pkgs)

.PHONY: test-race
test-race: fmt
	$(GO) test -v -count=1 -race $(pkgs)

.PHONY: test
test: unit integration

.PHONY: integration
integration:
	$(GO) test -v -count=1 -tags=integration ./test/integ-test

.PHONY: render-validation
render-validation:
	$(GO) test -v -count=1 ./test/render-validation

.PHONY: binaries
binaries: linux windows

.PHONY: linux
linux:
		GOOS=linux ${GO} build -o bin/shoelaces -ldflags ${LDFLAGS} ./cmd/shoelaces

.PHONY: windows
windows:
		GOOS=windows ${GO} build -o bin/shoelaces.exe -ldflags ${LDFLAGS} ./cmd/shoelaces

.PHONY: goreleaser-check
goreleaser-check:
	goreleaser check

.PHONY: goreleaser-snapshot
goreleaser-snapshot:
	goreleaser release --snapshot --clean

.PHONY: release-notes
release-notes:
ifndef RELEASE_TAG
	$(error RELEASE_TAG is required, e.g. make release-notes RELEASE_TAG=v1.4.0)
endif
	scripts/export-release-notes.sh "$(RELEASE_TAG)"
