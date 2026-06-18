# Contributing

## Development

Use the Go toolchain selected by `go.mod`.
The Nix development shell provides the expected local toolchain.

```bash
nix develop
go test ./...
go test -race ./...
go build -o shoelaces ./cmd/shoelaces
```

Useful make targets:

- `make` or `make all`: build the binary.
- `make unit`: run formatted Go unit tests.
- `make test`: run `make unit` and the historical Python integration tests.
- `make fmt`: format Go code.
- `make clean`: remove build artifacts.

The Python integration tests require the binary to be built first:

```bash
go build
./test/integ-test/integ_test.py -vv
```

Integration tests run on port `18888` and use fixtures in `test/integ-test/expected-results/`.

## Pull Requests

Keep changes small and self-reviewable.
Prefer focused commits that include the relevant tests or docs for the change.

Run the relevant tests for the area you changed when practical.
For broad changes, `go test ./...` is the baseline.

Prefer table-driven tests when they keep related cases easier to review.
Add comments for non-obvious production constraints, compatibility behavior, workarounds, performance tradeoffs, safety guardrails, or surprising behavior.
Avoid comments that restate the next line of code.

## Commit Titles

Use conventional commit titles.
The repository changelog and CI commit checks expect conventional commits.

Preferred commit types:

- `feat`
- `fix`
- `doc`
- `perf`
- `refactor`
- `style`
- `test`
- `chore`
- `ci`
- `revert`
- `security`

Examples:

```text
fix(cli): preserve config precedence
doc(agent): add repository guide
chore(go): bump toolchain to 1.26
```
