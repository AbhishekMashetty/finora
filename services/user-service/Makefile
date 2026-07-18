.PHONY: run build test tidy docker-build

run:
	go run ./cmd/server

build:
	go build ./...

test:
	go test ./...

tidy:
	go mod tidy

# Build context is the repo root (not this directory) because the image
# also needs the shared/ module. This target points docker at the repo
# root as context while still using this directory's Dockerfile.
docker-build:
	docker build -f Dockerfile -t user-service ../../
