# Tunkul
Grid-based node beat sequencer and drum machine

## Run server for pre-compiled wasm
make run-server

## Build
make all

## Dependencies
1. `Go` compiler (for building and running tests only)
2. `Python3` (for serving only)

### Environment setup for native Ebiten
If you want to build or run tests with the real Ebiten library, install the
required system packages first. A helper script is provided:

```sh
./scripts/setup-ebiten-env.sh
```

On macOS the script uses Homebrew, while on Linux it installs the necessary X11
and OpenGL libraries via `apt`. After running it you can build normally using
`make all`.

## Testing
Unit tests rely on the stubbed Ebiten API. Always run them with the alternate
module file so the stubs are used:

```sh
cd src/go
go test -tags test -modfile=go.test.mod ./...
```

Running `go test -tags test ./...` without `-modfile` tries to use the real
Ebiten library and will fail unless you have a full X11 environment installed.
