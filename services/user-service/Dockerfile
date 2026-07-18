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
RUN adduser -D -u 10001 appuser
COPY --from=builder /out/server /usr/local/bin/server
USER appuser
EXPOSE 8081
HEALTHCHECK --interval=10s --timeout=3s --retries=5 CMD wget -qO- http://localhost:8081/live || exit 1
ENTRYPOINT ["/usr/local/bin/server"]
