.PHONY: run build test tidy docker-build

run:
	go run ./cmd/server

build:
	go build -o bin/server ./cmd/server

test:
	go test ./...

tidy:
	go mod tidy

# Must be run from the repo root, since the Docker build context needs to
# include both shared/ and services/expense-service/:
#   make -C services/expense-service docker-build
docker-build:
	docker build -f Dockerfile -t finora/expense-service:local ../..
