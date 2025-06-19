# Tunkul
Grid-based node beat sequencer and drum machine

## Run server for pre-compiled wasm
make serve

## Build
make wasm
The build will automatically run `go mod download` to fetch dependencies the
first time, so network access is required on a fresh checkout.

## Dependencies
1. `Go` compiler (for building and running tests only)
2. `Python3` (for serving only)

### Environment setup for native Ebiten
If you want to build or run tests with the real Ebiten library, install the
required system packages first. A helper script is provided:

```sh
`make wasm`.
```

On macOS the script uses Homebrew, while on Linux it installs the necessary X11
and OpenGL libraries via `apt`. After running it you can build normally using
`make wasm`.

## Testing
Unit tests can run in two modes. Using the stubbed Ebiten API requires the
alternate module file:

```sh
make test-mock
```

If you have a working X11 setup (or run under `xvfb-run`) you can instead test
against the real Ebiten library:

```sh
make test-real
make test-xvfb
```

## Debugging
The UI and game layers now emit verbose logs describing user interactions and
internal state changes. Run the game from the repository root and check the
console output for messages prefixed with `[game]` and `[drumview]`.

### Headless browser tests
To experiment with UI automation you can attempt to run the WASM build inside a
headless browser. This requires a Chromium or Firefox binary. In this container
the packages depend on `snapd` which is not available, so headless tests cannot
run by default.

## Git hooks
Enable the optional pre-commit hook so every commit formats the code, runs the tests with the stubbed Ebiten module and builds the wasm binary:

```sh
git config core.hooksPath .githooks
```
