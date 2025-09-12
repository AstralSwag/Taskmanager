# Этап сборки
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Устанавливаем необходимые пакеты для сборки
RUN apk add --no-cache gcc musl-dev

# Копируем файлы зависимостей
COPY go.mod go.sum ./

# Устанавливаем зависимости
RUN go mod download

# Копируем исходный код
COPY . .

# Собираем приложение с включенным CGO
RUN CGO_ENABLED=1 GOOS=linux go build -o main .

# Финальный этап
FROM alpine:latest

WORKDIR /app

# Устанавливаем необходимые пакеты для работы с временными зонами
RUN apk --no-cache add ca-certificates tzdata

# Копируем собранное приложение из этапа сборки
COPY --from=builder /app/main .
COPY --from=builder /app/static ./static
COPY --from=builder /app/templates ./templates
COPY --from=builder /app/users.json ./users.json
COPY --from=builder /app/init-postgres.sql ./init-postgres.sql

EXPOSE 8080

CMD ["./main"] 