FROM golang:1.24.4-alpine AS builder
WORKDIR /app
COPY . .
COPY go.mod go.sum ./
RUN go mod download
WORKDIR /app/cmd/user-service
RUN go build -o user-service

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/cmd/user-service/user-service .
COPY --from=builder /app/cockroachdb cockroachdb
RUN apk --no-cache add ca-certificates && \
    adduser -D appuser && chown -R appuser:appuser /root
USER appuser
EXPOSE 50051
ENTRYPOINT ["./user-service"]
