FROM golang:1.24.4-alpine AS builder
WORKDIR /app
COPY . .
COPY go.mod go.sum ./
RUN go mod download
WORKDIR /app/cmd/telegram-event-listener
RUN go build -o telegram-event-listener

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/cmd/telegram-event-listener/telegram-event-listener .
COPY --from=builder /app/cockroachdb cockroachdb
RUN apk --no-cache add ca-certificates && \
    adduser -D appuser && chown -R appuser:appuser /root
USER appuser
EXPOSE 8080
ENTRYPOINT ["./telegram-event-listener"]
