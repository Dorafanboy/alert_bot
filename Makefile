.PHONY: build run test clean docker-build docker-run help

# Переменные
BINARY_NAME=bot
DOCKER_IMAGE=reminder-bot
DOCKER_CONTAINER=reminder-bot-container

# Цвета для вывода
GREEN := \033[0;32m
NC := \033[0m # No Color

help: ## Показать это сообщение
	@echo "Доступные команды:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "$(GREEN)%-30s$(NC) %s\n", $$1, $$2}'

build: ## Собрать приложение
	@echo "Сборка приложения..."
	go build -o $(BINARY_NAME) ./cmd/bot

run: ## Запустить приложение
	@echo "Запуск приложения..."
	go run ./cmd/bot

test: ## Запустить тесты
	@echo "Запуск тестов..."
	go test -v ./...

clean: ## Очистить собранные файлы
	@echo "Очистка..."
	rm -f $(BINARY_NAME)

docker-build: ## Собрать Docker образ
	@echo "Сборка Docker образа..."
	docker build -t $(DOCKER_IMAGE) .

docker-run: ## Запустить Docker контейнер
	@echo "Запуск Docker контейнера..."
	docker run --name $(DOCKER_CONTAINER) \
		--env-file .env \
		$(DOCKER_IMAGE)

docker-stop: ## Остановить Docker контейнер
	@echo "Остановка Docker контейнера..."
	docker stop $(DOCKER_CONTAINER) || true
	docker rm $(DOCKER_CONTAINER) || true

docker-clean: ## Удалить Docker образ
	@echo "Удаление Docker образа..."
	docker rmi $(DOCKER_IMAGE) || true

dev: ## Запустить в режиме разработки с hot-reload
	@echo "Запуск в режиме разработки..."
	air

setup: ## Настроить окружение разработки
	@echo "Настройка окружения разработки..."
	cp .env.example .env
	@echo "Создан файл .env. Пожалуйста, отредактируйте его и добавьте ваш токен бота." 