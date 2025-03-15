# Собираем образ приложения на Go
FROM golang:1.24 AS builder

# Устанавливаем рабочую директорию
WORKDIR /go/src/app

# Копируем исходный код
COPY . .

# Скачиваем зависимости
RUN go mod download

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -o /go/bin/app ./cmd/gateway

# Запускаем приложение в минимальном образе Alpine
FROM alpine:3.18

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем собранное приложение из стадии builder
COPY --from=builder /go/bin/app /app/app

# Убедимся, что файл исполняемый
RUN chmod +x /app/app

# Объявляем порт
EXPOSE 8080

# Запускаем приложение
CMD ["./app"]