# Build context MUST be the repo root, e.g.:
#   docker build -f services/notification-service/Dockerfile .
# (needed so this stage can COPY the sibling shared/ module).
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY shared/ ./shared/
COPY services/notification-service/ ./services/notification-service/
WORKDIR /app/services/notification-service
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd/server

FROM alpine:3.19
RUN adduser -D -u 10001 appuser
WORKDIR /app
COPY --from=builder /out/server /usr/local/bin/server
# openapi.yaml is served live at GET /openapi.yaml (Phase 6) — see
# shared/openapidoc's doc comment for why it's copied from disk rather than
# go:embed'd, and why WORKDIR matches native `go run` from this directory.
COPY --from=builder /app/services/notification-service/openapi.yaml ./openapi.yaml
RUN chown appuser:appuser /app/openapi.yaml
USER appuser
EXPOSE 8084
HEALTHCHECK --interval=10s --timeout=3s --retries=5 CMD wget -qO- http://localhost:8084/live || exit 1
ENTRYPOINT ["/usr/local/bin/server"]
