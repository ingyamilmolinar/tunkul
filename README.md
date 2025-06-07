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
Run unit tests with the Ebiten stubs enabled:

```sh
go test -tags test ./core/... ./internal/ui
```

Using the real Ebiten library requires an X11 environment with the appropriate dev packages installed; tests without the `test` tag will fail on a headless machine.
