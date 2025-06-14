WASM_OUT = ../js/main.wasm

build-wasm:
	cd src/go && go mod download
	cd src/go && GOOS=js GOARCH=wasm go build -o $(WASM_OUT) ./cmd/...

run-server:
	# simplest local server for testing:
	cd src/js && python3 -m http.server 8080

all: build-wasm

desktop:
	cd src/go; go run ./cmd/tunkul
