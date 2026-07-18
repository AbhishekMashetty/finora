.PHONY: run build test tidy docker-build

run:
	go run ./cmd/server

build:
	go build -o bin/server ./cmd/server

test:
	go test ./...

tidy:
	go mod tidy

docker-build:
	docker build -f Dockerfile -t finora/gateway ../../
