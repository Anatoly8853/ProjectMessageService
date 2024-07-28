FROM golang:1.20-alpine

WORKDIR /app

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .

# Сборка исполняемого файла
RUN go build -o ProjectMessageService ./cmd/main.go

# Проверка наличия файла и его прав после сборки
RUN ls -l /app

EXPOSE 8080

# Запуск исполняемого файла
CMD ["./ProjectMessageService"]