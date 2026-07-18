.PHONY: run build test tidy docker-build

run:
	go run ./cmd/server

build:
	go build ./...

test:
	go test ./...

tidy:
	go mod tidy

# The build context must be the REPO ROOT (not this directory), since the
# Dockerfile also COPYs shared/. This target cd's there for you; equivalent
# to running, from the repo root:
#   docker build -f services/budget-service/Dockerfile -t budget-service:local .
docker-build:
	cd $(CURDIR)/../.. && docker build -f services/budget-service/Dockerfile -t budget-service:local .
