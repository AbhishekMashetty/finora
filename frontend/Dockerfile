FROM node:20-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
# NEXT_PUBLIC_* vars are inlined into the client bundle at build time, so this
# must be a build ARG (not a runtime `environment:` in docker-compose).
ARG NEXT_PUBLIC_API_BASE_URL=http://localhost:8080
ENV NEXT_PUBLIC_API_BASE_URL=${NEXT_PUBLIC_API_BASE_URL}
RUN npm run build

FROM node:20-alpine AS runner
WORKDIR /app
RUN addgroup -g 1001 -S nodejs && adduser -S nextjs -u 1001
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static
COPY --from=builder /app/public ./public
USER nextjs
EXPOSE 3000
ENV PORT=3000
# Docker sets HOSTNAME to the container ID by default, and Next's standalone
# server.js binds to `process.env.HOSTNAME || '0.0.0.0'` — without this
# override it ends up listening only on the container's own interface IP,
# not loopback, so `localhost` healthchecks (and anything using loopback)
# get connection refused even though host port-mapping still works via NAT.
ENV HOSTNAME=0.0.0.0
# Use 127.0.0.1, not localhost: /etc/hosts resolves localhost to ::1 first,
# and the server only listens on IPv4 0.0.0.0, so an IPv6 attempt gets
# connection refused even though the IPv4 path is healthy.
HEALTHCHECK --interval=10s --timeout=3s --retries=5 CMD wget -qO- http://127.0.0.1:3000 || exit 1
CMD ["node", "server.js"]
