SERVICES := gateway user-service expense-service budget-service notification-service

.PHONY: up down logs build test tidy ps clean-volumes

## Bring up the full stack (build images first).
up:
	docker compose up --build

## Bring up the full stack detached.
up-d:
	docker compose up --build -d

## Stop and remove containers (keeps data volumes).
down:
	docker compose down

## Tail logs for every service.
logs:
	docker compose logs -f

ps:
	docker compose ps

## Build every Go module + the frontend, without Docker.
build:
	@for s in shared $(addprefix services/,$(SERVICES)); do \
		echo "== building $$s =="; \
		(cd $$s && go build ./...) || exit 1; \
	done
	cd frontend && npm run build

## Run unit tests for every Go module.
test:
	@for s in shared $(addprefix services/,$(SERVICES)); do \
		echo "== testing $$s =="; \
		(cd $$s && go test ./...) || exit 1; \
	done

## go mod tidy every Go module.
tidy:
	@for s in shared $(addprefix services/,$(SERVICES)); do \
		echo "== tidying $$s =="; \
		(cd $$s && go mod tidy) || exit 1; \
	done

## Danger: also drop the Mongo data volumes.
clean-volumes:
	docker compose down -v
