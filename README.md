# Plans Tracker backend

Минимальная основа Go-бекенда для мобильного приложения. Сервисы `tasks` и `habits` запускаются отдельно, используют отдельные базы данных в одном PostgreSQL и общий Redis. Пока они предоставляют только проверки состояния. SQL-схемы для текущих данных приложения лежат в `docker/postgres/migrations`; предметные API остаются за разработчиком бекенда.

## Запуск

Нужны Docker и Docker Compose. Из этой папки:

```sh
cp .env.example .env
docker compose up --build -d
curl -i http://localhost:8081/health/ready
curl -i http://localhost:8082/health/ready
```

Сервисы доступны на `localhost:8081` и `localhost:8082`. PostgreSQL доступен локально на порту `5433`, Redis на `6380`. Все опубликованные порты привязаны только к `127.0.0.1`. Локально PostgreSQL хранит данные в слое контейнера, Redis не сохраняет их на диск. `docker compose down` удаляет контейнер и локальную базу. Для очистки также созданных образом анонимных томов используйте `docker compose down -v`.

## Хранение данных в продакшене

Файл `compose.prod.yaml` добавляет постоянный том PostgreSQL. Задайте отдельный `PROD_POSTGRES_PASSWORD` в окружении или в `.env` и запускайте с обоими файлами:

```sh
docker compose -p plans-prod -f compose.yaml -f compose.prod.yaml up --build -d
docker compose -p plans-prod -f compose.yaml -f compose.prod.yaml down
```

Том сохраняется после перезапуска или пересоздания контейнера и после обычного `down`. Не применяйте к продакшену `down -v`: эта команда удалит том с базой. Том не заменяет резервные копии и не защищает от потери диска сервера. Redis в обеих конфигурациях остаётся временным хранилищем.

`.env.example` содержит пароль только для локального запуска. Перед использованием вне локальной машины задайте свой `PROD_POSTGRES_PASSWORD` и настройте доступ к Redis. В продакшене пароль передаётся приложению через `DATABASE_PASSWORD`, отдельно от URL.

## Схема PostgreSQL

`docker/postgres/init.sql` создаёт базы `accounts`, `tasks`, `habits` и применяет SQL-файлы из `docker/postgres/migrations` при первом запуске PostgreSQL. Повторный запуск скрипта пропускает уже применённую версию `001_initial`. Если том PostgreSQL был создан до появления этих таблиц, примените схему вручную после запуска контейнера:

```sh
docker compose exec -T postgres psql -U app -d postgres -f /docker-entrypoint-initdb.d/01-init.sql
```

Для продакшен-стека добавьте к команде `-p plans-prod -f compose.yaml -f compose.prod.yaml` перед `exec`.

Таблицы соответствуют текущим локальным моделям приложения:

- `accounts`: `users`, настройки приложения, календаря, оформления и уведомлений;
- `tasks`: статусы, приоритеты, группы, задачи, статусы повторений, отметки выполнения, события календаря;
- `habits`: категории, привычки, отметки выполнения.

У каждого пользователя свои статусы и приоритеты. При создании первой задачи базовые значения появляются автоматически. Чтобы получить их до первой задачи, вызовите в базе `tasks` функцию `SELECT ensure_task_defaults('<user-uuid>');`. Ссылки `user_id` из баз `tasks` и `habits` на `accounts.users` не имеют внешнего ключа, поскольку базы раздельны; создание и удаление связанных данных должен координировать бекенд. Ссылки событий календаря на привычки также проверяются бекендом. Таблица `users` не задаёт способ входа и не хранит пароли: авторизация пока не определена. Значение `notification_settings.enabled` отражает пользовательскую настройку, а фактическое разрешение ОС остаётся на устройстве.

## Локальный запуск Go

После запуска зависимостей через `docker compose up -d postgres redis`:

```sh
DATABASE_URL='postgres://app:localdev@localhost:5433/tasks?sslmode=disable' REDIS_ADDR='localhost:6380' PORT=8081 go run ./cmd/tasks
DATABASE_URL='postgres://app:localdev@localhost:5433/habits?sslmode=disable' REDIS_ADDR='localhost:6380' PORT=8082 go run ./cmd/habits
```

Обязательны `DATABASE_URL` и `REDIS_ADDR`; опциональны `DATABASE_PASSWORD`, `REDIS_PASSWORD` и `PORT` (по умолчанию `8080`). `GET /health/live` проверяет процесс, `GET /health/ready` проверяет PostgreSQL и Redis. Контракт описан в `openapi.yaml`.

## Структура

- `cmd/tasks`, `cmd/habits` — отдельные исполняемые сервисы.
- `internal/platform` — конфигурация, подключение зависимостей и HTTP-сервер.
- `docker/postgres/init.sql` — создание баз и применение SQL-миграций.
- `docker/postgres/migrations` — версии схем для каждой базы.
- `openapi.yaml` — текущий HTTP-контракт.

Мобильное приложение сейчас хранит данные локально. Эта основа пока не подключена к нему и не определяет серверную модель задач и привычек.
