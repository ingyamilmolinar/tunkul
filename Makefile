WASM_OUT = ../js/main.wasm

# compatibility aliases
wasm: build-wasm
serve: run-server

run:
	cd src/go; go run ./cmd/tunkul

test-mock:
	cd src/go; go test -tags test -modfile=go.test.mod ./...

test-real:
	cd src/go; go test -tags test ./...

test:
	cd src/go; go test -tags test -modfile=go.test.mod ./...
	cd src/go; go test -tags test ./...

test-xvfb:
	cd src/go; xvfb-run go test -tags test ./...

dependencies:
	./scripts/setup-ebiten-env.sh

wasm:
	cd src/go && go mod download
	cd src/go && GOOS=js GOARCH=wasm go build -o $(WASM_OUT) ./cmd/...

serve:
	# simplest local server for testing:
	cd src/js && python3 -m http.server 8080

run:
	cd src/go; go run ./cmd/tunkul

test-mock:
	cd src/go; go test -tags test -modfile=go.test.mod ./...

test-real:
	cd src/go; go test -tags test ./...

test:
	cd src/go; go test -tags test -modfile=go.test.mod ./...
	cd src/go; go test -tags test ./...

test-xvfb:
	cd src/go; xvfb-run go test -tags test ./...

dependencies:
	./scripts/setup-ebiten-env.sh

