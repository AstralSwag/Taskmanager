# Этап сборки
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Копируем файлы зависимостей
COPY go.mod go.sum ./

# Устанавливаем зависимости
RUN go mod download

# Копируем исходный код
COPY . .

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# Финальный этап
FROM alpine:latest

WORKDIR /app

# Копируем собранное приложение из этапа сборки
COPY --from=builder /app/main .
COPY --from=builder /app/static ./static
COPY --from=builder /app/templates ./templates

# Устанавливаем необходимые пакеты
RUN apk --no-cache add ca-certificates

EXPOSE 8080

CMD ["./main"] 