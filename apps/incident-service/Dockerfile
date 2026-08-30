# syntax=docker/dockerfile:1.7

FROM golang:1.25-alpine AS builder

WORKDIR /src

# Cache modules separately from source code.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build \
      -trimpath \
      -ldflags="-s -w" \
      -o /out/incident-service \
      ./apps/incident-service/cmd/incident-service

FROM alpine:3.21 AS runtime

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S -g 10001 sre \
    && adduser -S -D -H -u 10001 -G sre sreuser

WORKDIR /app
COPY --from=builder /out/incident-service /app/incident-service

USER 10001:10001

EXPOSE 8081

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:8081/actuator/health/readiness >/dev/null || exit 1

ENTRYPOINT ["/app/incident-service"]
