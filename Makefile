BINARY      := bin/api
LAMBDA_BIN  := bin/bootstrap
LAMBDA_ZIP  := bin/lambda.zip
PORT        ?= 8080

# Load .env.local if present (sets AWS_ENDPOINT_URL → LocalStack, etc.)
-include .env.local
export

.PHONY: run build lambda-build clean tidy

## run: start the local dev server (Ctrl-C to stop)
run:
	PORT=$(PORT) go run ./cmd/api

## build: compile for the current platform
build:
	@mkdir -p bin
	go build -o $(BINARY) ./cmd/api
	@echo "Built: $(BINARY)"

## lambda-build: cross-compile for AWS Lambda (arm64, provided.al2023)
lambda-build:
	@mkdir -p bin
	GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o $(LAMBDA_BIN) ./cmd/api
	zip -j $(LAMBDA_ZIP) $(LAMBDA_BIN)
	@echo "Lambda package: $(LAMBDA_ZIP)  (upload this to Lambda)"

## clean: remove build artifacts
clean:
	rm -rf bin

## tidy: sync go.mod / go.sum
tidy:
	go mod tidy
