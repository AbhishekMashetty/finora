# Build context MUST be the repo root (not services/user-service), because
# this Dockerfile also needs the shared/ module. Build from repo root with:
#
#   docker build -f services/user-service/Dockerfile .
#
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY shared/ ./shared/
COPY services/user-service/ ./services/user-service/
WORKDIR /app/services/user-service
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd/server

FROM alpine:3.19
# tzdata is required for time.LoadLocation to resolve any IANA zone other
# than UTC — Alpine ships without it, so settings' timezone validation
# (internal/service/user_service.go's UpdateSettings) would silently reject
# every real timezone except "UTC" without this. Caught live: a genuinely
# valid zone (Europe/London) was rejected with the same 400 VALIDATION_ERROR
# a garbage string gets, because LoadLocation itself failed to find zone
# data on disk, not because the input was actually invalid.
RUN apk add --no-cache tzdata
RUN adduser -D -u 10001 appuser
WORKDIR /app
COPY --from=builder /out/server /usr/local/bin/server
# openapi.yaml is served live at GET /openapi.yaml (Phase 6) — see
# shared/openapidoc's doc comment for why it's copied from disk rather than
# go:embed'd, and why WORKDIR matches native `go run` from this directory.
COPY --from=builder /app/services/user-service/openapi.yaml ./openapi.yaml
RUN chown appuser:appuser /app/openapi.yaml
USER appuser
EXPOSE 8081
HEALTHCHECK --interval=10s --timeout=3s --retries=5 CMD wget -qO- http://localhost:8081/live || exit 1
ENTRYPOINT ["/usr/local/bin/server"]
