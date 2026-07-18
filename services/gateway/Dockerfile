# Build context MUST be the repo root, e.g.:
#   docker build -f services/gateway/Dockerfile -t finora/gateway .
# (the gateway needs shared/ alongside it — see go.mod's local `replace`.)

FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY shared/ ./shared/
COPY services/gateway/ ./services/gateway/
WORKDIR /app/services/gateway
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd/server

FROM alpine:3.19
RUN adduser -D -u 10001 appuser
COPY --from=builder /out/server /usr/local/bin/server
USER appuser
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --retries=5 CMD wget -qO- http://localhost:8080/live || exit 1
ENTRYPOINT ["/usr/local/bin/server"]
