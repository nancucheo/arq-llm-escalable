FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY . .
RUN go mod tidy \
    && CGO_ENABLED=0 GOOS=linux go build -o cache-cleaner  ./cmd/cache-cleaner \
    && CGO_ENABLED=0 GOOS=linux go build -o usage-reporter ./cmd/usage-reporter

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/cache-cleaner  .
COPY --from=builder /app/usage-reporter .
# No CMD — the K8s CronJob specifies the binary via `command`
