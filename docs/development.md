# Development

The Go module lives under `src`. Use Go 1.25 or newer, with current security patches.
There are no third-party runtime modules or broker/database dependencies.

## Build

From the repository root:

```bash
cd src
go build -o ../bin/hvx ./cmd/hvx
```

Cross-compile from Linux using `GOOS=windows GOARCH=amd64` or
`GOOS=darwin GOARCH=arm64` before the same build command, with an appropriate output
filename. Native Windows builds use `go build -o ../bin/hvx.exe ./cmd/hvx`.

## Checks

From `src`:

```bash
gofmt -l .
go vet ./...
go test -race -count=1 ./...
go build ./...
```

Formatting should report no files. Tests use local HTTP servers and temporary
configuration directories. They cover request paths and credentials, idempotency
headers and stable retry payloads, redirect refusal, task decoding, polling deadlines,
mode isolation, and legacy configuration migration. No test contacts real hosts or
uses a production PAT.

## Continuous integration

[CLI checks](../.github/workflows/cli.yml) runs on pushes and pull requests. It checks
formatting, static analysis, tests with the race detector, and builds on Ubuntu and
Windows. A separate job cross-compiles Linux amd64, macOS arm64, and Windows amd64
binaries. CI selects the latest patch in the Go 1.25 series.

The workflow does not publish binaries or deploy anything. Cross-compilation does
not prove runtime behavior on macOS. Full validation against a deployed OpenHVX
backend remains a separate integration step.
