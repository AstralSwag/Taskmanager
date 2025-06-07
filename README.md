# Task Manager

Простое веб-приложение для управления задачами, написанное на Go с использованием PostgreSQL.

## Требования

- Go 1.21 или выше
- PostgreSQL (локальный или удаленный)
- Git
- Docker и Docker Compose (опционально)

## Установка

1. Клонируйте репозиторий:
```bash
git clone <repository-url>
cd taskmanager
```

2. Установите зависимости:
```bash
go mod download
```

3. Настройка базы данных:
   - Создайте файл .env в корне проекта со следующими параметрами:
   ```
   DB_HOST=your_remote_host
   DB_PORT=5432
   DB_USER=your_username
   DB_PASSWORD=your_password
   DB_NAME=your_database_name
   DB_SSLMODE=require
   ```
   - Замените значения на ваши реальные данные для подключения к базе данных

4. Запуск приложения:
   - Нативно:
   ```bash
   go run main.go
   ```
   - С помощью Docker:
   ```bash
   docker-compose up --build
   ```

Приложение будет доступно по адресу: http://localhost:8080

## Функциональность

- Отображение списка задач в виде карточек
- Красивый современный интерфейс
- Адаптивный дизайн
- Индикация статуса задачи цветом
- Возможность создания плана и факта
- Генерация markdown таблиц
- Комментарии к задачам 