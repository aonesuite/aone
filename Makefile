.PHONY: build install clean fmt test unittest integrationtest staticcheck generate generate-sandbox

# --- CLI ---------------------------------------------------------------------

# Output binary for `make build`. Override with `make build OUT=/usr/local/bin/aone`.
OUT ?= ./aone

# Build the aone CLI binary.
build:
	go build -o $(OUT) .

# Install the aone CLI into $GOPATH/bin (or $GOBIN).
install:
	go install .

# Remove the local binary produced by `make build`.
clean:
	rm -f ./aone

# Format Go source files.
fmt:
	gofmt -w .

# --- Tests / codegen ---------------------------------------------------------

test:
	go test -failfast -count=1 -v -timeout 30m ./...

unittest:
	go test -failfast -count=1 -v ./...

integrationtest:
	go test -failfast -count=1 -parallel 1 -v ./...

staticcheck:
	staticcheck ./...

generate: generate-sandbox

generate-sandbox:
	# Specs are synced by infra task before generation.
	# Control plane API
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.1 \
		--config packages/go/sandbox/internal/apis/oapi-codegen.yaml \
		api/sandbox/openapi.yml
	# Volume content API
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.1 \
		--config packages/go/sandbox/internal/volumeapi/oapi-codegen.yaml \
		api/sandbox/openapi-volumecontent.yml
	# envd HTTP API
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.1 \
		--config packages/go/sandbox/internal/envdapi/oapi-codegen.yaml \
		api/sandbox/envd/envd.yaml
	# envd ConnectRPC (requires buf, protoc-gen-go, protoc-gen-connect-go)
	cd api/sandbox/envd && buf generate
	# Format and verify generated sandbox package
	gofmt -w packages/go/sandbox/internal/apis packages/go/sandbox/internal/volumeapi packages/go/sandbox/internal/envdapi
	go build ./packages/go/sandbox/...
