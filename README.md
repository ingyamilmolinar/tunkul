# Tunkul
Grid-based node beat sequencer and drum machine

## Run server for pre-compiled wasm
make run-server

## Build
make all

## Dependencies
1. `Go` compiler (for building and running tests only)
2. `Python3` (for serving only)

## Testing
Unit tests rely on the stubbed Ebiten API. Always run them with the alternate
module file so the stubs are used:

```sh
cd src/go
go test -tags test -modfile=go.test.mod ./...
```

Running `go test -tags test ./...` without `-modfile` tries to use the real
Ebiten library and will fail unless you have a full X11 environment installed.
