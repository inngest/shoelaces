GO = go
SCDOC = scdoc
LDFLAGS = "-s -w"

pkgs = ./...

all:
	$(GO) build ./cmd/shoelaces

fmt:
	$(GO) fmt ./...

clean:
	rm -f shoelaces docs/shoelaces.8

shoelaces.8:
	$(SCDOC) < docs/shoelaces.8.scd > docs/shoelaces.8

docs: shoelaces.8

unit: fmt
	$(GO) test -v -count=1 $(pkgs)

test: unit
	./test/integ-test/integ_test.py -vv

.PHONY: all clean docs unit test

binaries: linux windows
linux:
		GOOS=linux ${GO} build -o bin/shoelaces -ldflags ${LDFLAGS} ./cmd/shoelaces
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
	$(error RELEASE_TAG is required, e.g. make release-notes RELEASE_TAG=v2026-05-07.01)
endif
	scripts/export-release-notes.sh "$(RELEASE_TAG)"
