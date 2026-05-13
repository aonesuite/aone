.PHONY: build install clean fmt test unittest integrationtest staticcheck generate generate-aone generate-sandbox releasecheck

# --- CLI ---------------------------------------------------------------------

# Output binary for `make build`. Override with `make build OUT=/usr/local/bin/aone`.
OUT ?= ./bin/aone
OAPI_CODEGEN ?= go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.1

# Build the aone CLI binary.
build:
	mkdir -p $(dir $(OUT))
	go build -o $(OUT) .

# Install the aone CLI into $GOPATH/bin (or $GOBIN).
install:
	go install .

# Remove the local binary produced by `make build`.
clean:
	rm -rf ./bin
	rm -f ./aone

# Format Go source files.
fmt:
	gofmt -w .

# --- Tests / codegen ---------------------------------------------------------

test:
	go test -failfast -count=1 -v -timeout 30m ./...
	cd packages/go && go test -failfast -count=1 -v -timeout 30m ./...

unittest:
	go test -failfast -count=1 -v ./...
	cd packages/go && go test -failfast -count=1 -v ./...

integrationtest:
	go test -failfast -count=1 -parallel 1 -v ./...
	cd packages/go && go test -failfast -count=1 -parallel 1 -v ./...

staticcheck:
	staticcheck ./...
	cd packages/go && staticcheck ./...

releasecheck:
	@if grep -q 'github.com/aonesuite/aone/packages/go v0\.0\.0' go.mod; then \
		echo "releasecheck: root go.mod still depends on Go SDK v0.0.0; tag packages/go/vX.Y.Z and require it before releasing the CLI"; \
		exit 1; \
	fi
	@if grep -q '^replace github.com/aonesuite/aone/packages/go => ./packages/go' go.mod; then \
		echo "releasecheck: root go.mod still has a local Go SDK replace; remove it before releasing the CLI"; \
		exit 1; \
	fi
	GOWORK=off go test -failfast -count=1 ./...

generate: generate-aone generate-sandbox

generate-aone:
	# Full Aone service API
	$(OAPI_CODEGEN) --config packages/go/internal/aoneapi/oapi-codegen.yaml \
		spec/openapi.yml

	# Format and verify generated Aone API package
	gofmt -w packages/go/internal/aoneapi
	cd packages/go && go build ./internal/aoneapi

generate-sandbox:
	# Volume content API
	$(OAPI_CODEGEN) --config packages/go/sandbox/internal/volumeapi/oapi-codegen.yaml \
		spec/sandbox/openapi-volumecontent.yml

	# envd HTTP API
	$(OAPI_CODEGEN) --config packages/go/sandbox/internal/envdapi/oapi-codegen.yaml \
		spec/sandbox/envd/envd.yaml

	# envd ConnectRPC (requires buf, protoc-gen-go, protoc-gen-connect-go)
	cd spec/sandbox/envd && buf generate

	# Format and verify generated sandbox-specific packages
	gofmt -w packages/go/sandbox/internal/volumeapi packages/go/sandbox/internal/envdapi
	cd packages/go && go build ./...
