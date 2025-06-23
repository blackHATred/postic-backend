FROM golang:1.24.4-alpine AS builder
WORKDIR /app
COPY . .
COPY go.mod go.sum ./
RUN go mod download
WORKDIR /app/cmd/upload-service
RUN go build -o upload-service

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/cmd/upload-service/upload-service .
COPY --from=builder /app/cockroachdb cockroachdb
RUN apk --no-cache add ca-certificates && \
    adduser -D appuser && chown -R appuser:appuser /root
USER appuser
EXPOSE 50052
ENTRYPOINT ["./upload-service"]
