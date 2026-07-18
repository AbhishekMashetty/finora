# Build context MUST be the repo root, not this directory, because this
# service depends on the shared/ Go module via a local `replace` directive.
# Build from the repo root with:
#   docker build -f services/expense-service/Dockerfile .
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY shared/ ./shared/
COPY services/expense-service/ ./services/expense-service/
WORKDIR /app/services/expense-service
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd/server

FROM alpine:3.19
RUN adduser -D -u 10001 appuser
WORKDIR /app
COPY --from=builder /out/server /usr/local/bin/server
# openapi.yaml is served live at GET /openapi.yaml (Phase 6) — copied
# straight from the builder stage (which already has it from the context
# copy above), not fetched separately, so there's exactly one source of
# truth for the file's contents. WORKDIR /app makes the server's cwd match
# where a native `go run`/`make run` from services/expense-service/ would
# already find it, so the handler can use one relative path ("openapi.yaml")
# in both environments — no environment-specific logic.
COPY --from=builder /app/services/expense-service/openapi.yaml ./openapi.yaml
RUN chown appuser:appuser /app/openapi.yaml
USER appuser
EXPOSE 8082
HEALTHCHECK --interval=10s --timeout=3s --retries=5 CMD wget -qO- http://localhost:8082/live || exit 1
ENTRYPOINT ["/usr/local/bin/server"]
