.PHONY: build run test fuzz bench lint docker-build proto

BINARY_NAME=matcher-server
MAIN_PATH=./cmd/grpc_server/main.go

## build: Compile the gRPC server binary
build:
	go build -o bin/$(BINARY_NAME)$(MAIN_PATH)

## run: Run the gRPC server locally
run:
	CONFIG_PATH=./config.yaml go run $(MAIN_PATH)

## test: Run unit tests with race detector and coverage
test:
	go test -v -race -cover ./...

## fuzz: Run fuzzing tests on the similarity calculator for 10 seconds
fuzz:
	go test -fuzz=FuzzTwoPassSimilarityCheck -fuzztime=10s ./similarity

## bench: Run performance and memory allocation benchmarks
bench:
	go test -bench=. -benchmem ./similarity

## lint: Run golangci-lint
lint:
	golangci-lint run ./...

## proto: Regenerate gRPC protobuf code
proto:
	protoc --go_out=. --go-grpc_out=. proto/market.proto

## docker-build: Build lightweight production Docker image
docker-build:
	docker build -t prediction-market-matcher:latest .