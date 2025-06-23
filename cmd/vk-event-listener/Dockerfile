FROM golang:1.24.4-alpine AS builder
WORKDIR /app
COPY . .
COPY go.mod go.sum ./
RUN go mod download
WORKDIR /app/cmd/vk-event-listener
RUN go build -o vk-event-listener

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/cmd/vk-event-listener/vk-event-listener .
COPY --from=builder /app/cockroachdb cockroachdb
RUN apk --no-cache add ca-certificates && \
    adduser -D appuser && chown -R appuser:appuser /root
USER appuser
EXPOSE 8080
ENTRYPOINT ["./vk-event-listener"]
