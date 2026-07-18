.PHONY: run build test tidy docker-build

run:
	go run ./cmd/server

build:
	go build -o bin/server ./cmd/server

test:
	go test ./...

tidy:
	go mod tidy

# Build context is the repo root (this Dockerfile COPYs the sibling shared/
# module), so this target intentionally cds up two levels before invoking
# docker build. Run `make docker-build` from within this directory.
docker-build:
	cd ../.. && docker build -f services/notification-service/Dockerfile -t notification-service .
