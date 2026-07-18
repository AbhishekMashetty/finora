# Multi-stage build. IMPORTANT: build context is the REPO ROOT, not this
# directory, because this stage needs to COPY the shared/ module too:
#
#   docker build -f services/budget-service/Dockerfile .
#
# (run from the repository root).

FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY shared/ ./shared/
COPY services/budget-service/ ./services/budget-service/
WORKDIR /app/services/budget-service
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd/server

FROM alpine:3.19
RUN adduser -D -u 10001 appuser
COPY --from=builder /out/server /usr/local/bin/server
USER appuser
EXPOSE 8083
HEALTHCHECK --interval=10s --timeout=3s --retries=5 CMD wget -qO- http://localhost:8083/live || exit 1
ENTRYPOINT ["/usr/local/bin/server"]
