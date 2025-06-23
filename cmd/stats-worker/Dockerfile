FROM golang:1.24.4-alpine AS builder
WORKDIR /app
COPY . .
COPY go.mod go.sum ./
RUN go mod download
WORKDIR /app/cmd/stats-worker
RUN go build -o stats-worker

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/cmd/stats-worker/stats-worker .
COPY --from=builder /app/cockroachdb cockroachdb
RUN apk --no-cache add ca-certificates && \
    adduser -D appuser && chown -R appuser:appuser /root
USER appuser
EXPOSE 8080
ENTRYPOINT ["./stats-worker"]
