# Plans Tracker backend

Go-бекенд для мобильного приложения. Сервисы `tasks` и `habits` запускаются отдельно, используют отдельные базы данных в одном PostgreSQL и общий Redis. В сервисе `tasks` есть CRUD задач, статусов, приоритетов и групп. SQL-схемы лежат в `docker/postgres/migrations`.

## Запуск

Нужны Docker и Docker Compose. Из этой папки:

```sh
cp .env.example .env
docker compose up --build -d
curl -i http://localhost:8081/health/ready
curl -i http://localhost:8082/health/ready
```

Обычный локальный запуск автоматически подключает `compose.override.yaml`: при первом создании PostgreSQL он добавляет тестового пользователя и четыре задачи из `docker/postgres/seed.dev.sql`. Продакшен-команда ниже явно указывает Compose-файлы и не подключает dev seed.

Тестовый пользователь: `8dc53fe1-0f08-4bd0-a3ed-6a5541859490` (`dev@example.invalid`). Сервис `tasks` читает этот ID из `DEV_USER_ID` для всех ручек задач и справочников. Если локальная база была создана раньше, добавьте записи вручную из корня проекта:

```sh
docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app -d postgres < docker/postgres/seed.dev.sql
```

Seed можно запускать повторно: существующие записи он не дублирует. Прежде чем запускать его на старой базе, примените SQL-схему командой из раздела «Схема PostgreSQL».

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

`docker/postgres/init.sql` создаёт базы `accounts`, `tasks`, `habits` и применяет SQL-файлы из `docker/postgres/migrations` при первом запуске PostgreSQL. Повторный запуск скрипта пропускает уже применённые версии. Если том PostgreSQL уже существует, запустите скрипт вручную после запуска контейнера, чтобы применить новые миграции:

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
DATABASE_URL='postgres://app:localdev@localhost:5433/tasks?sslmode=disable' REDIS_ADDR='localhost:6380' DEV_USER_ID='8dc53fe1-0f08-4bd0-a3ed-6a5541859490' PORT=8081 go run ./cmd/tasks
DATABASE_URL='postgres://app:localdev@localhost:5433/habits?sslmode=disable' REDIS_ADDR='localhost:6380' PORT=8082 go run ./cmd/habits
```

Обязательны `DATABASE_URL` и `REDIS_ADDR`; опциональны `DATABASE_PASSWORD`, `REDIS_PASSWORD` и `PORT` (по умолчанию `8080`). `GET /health/live` проверяет процесс, `GET /health/ready` проверяет PostgreSQL и Redis. Контракт описан в `openapi.yaml`.

## Каркас сервиса tasks

`cmd/tasks/main.go` соединяет слои. `internal/platform` открывает PostgreSQL, Redis и HTTP-сервер, затем вызывает регистрацию маршрутов сервиса. Код задач расположен отдельно:

- `internal/tasks/task.go` — начальная доменная модель. Дополняйте её по мере реализации функций.
- `internal/tasks/service.go` — интерфейс хранилища и место для правил работы с задачами.
- `internal/tasks/postgres/repository.go` — место для SQL-запросов к базе `tasks`.
- `internal/tasks/httpapi/handler.go` — REST-маршруты и обработчики.
- `internal/tasks/httpapi/dto.go` — место для структур JSON-запросов и ответов.

`GET /v1/tasks?start_date=2026-10-10&end_date=2026-10-20` возвращает `{"items":[...]}` для `DEV_USER_ID`. Обе даты обязательны, формат — `YYYY-MM-DD`, максимум 366 дней. В список входят задачи хотя бы с одним запланированным днём в диапазоне; повторяющаяся задача возвращается один раз, даже если началась раньше. Каждый элемент содержит вложенный `status`; `priority` и `group` содержат объекты или `null`, если не заданы. `GET /v1/tasks/backlog` возвращает задачи без дат и повторения. Схема задачи в мобильном приложении находится в `../plans-tracker-mobile/src/shared/api/tasks/model.ts`.

`GET /v1/task-occurrences?start_date=2026-10-10&end_date=2026-10-20` возвращает отдельный элемент для каждого дня задачи. У элемента есть `task`, `occurrence_date`, `status`, `completed`, `completed_at`. Повторения `daily`, `weekly` (`daysOfWeek`: 0 — воскресенье), `monthly` (`dayOfMonth`) и `custom` (`everyNDays`) считаются от `start_date`, а если её нет — от даты создания задачи по UTC. Исходный `end_date` не ограничивает будущие повторения; для неповторяющейся задачи учитывается весь диапазон от `start_date` до `end_date`. Несуществующие даты месяца пропускаются.

`GET /v1/tasks/{id}/occurrences/{date}` читает одно появление. Для повторяющейся задачи `PUT /v1/tasks/{id}/occurrences/{date}/status` с `{"status_id":"status-done"}` задаёт статус только на эту дату и создаёт отметку выполнения; любой статус с `is_completed=true` работает так же. `PUT /v1/tasks/{id}/occurrences/{date}/completion` с `{"completed":false}` снимает отметку и сбрасывает завершённый статус на `status-todo`. Для неповторяющейся задачи меняйте общий статус через `PATCH /v1/tasks/{id}`; запись статуса отдельного появления вернёт `409`.

`GET /v1/tasks/{id}` возвращает одну задачу в том же формате, включая задачи без дат. Для отсутствующей или чужой задачи возвращается `404`.

`POST /v1/tasks` создаёт задачу и возвращает `201` с заголовком `Location`. Обязателен `title`; по умолчанию используются `status-todo` и 60 минут. Если передана только одна из дат, она копируется в другую. `PATCH /v1/tasks/{id}` изменяет только переданные поля; чтобы изменить или очистить диапазон дат, передайте обе даты. При изменении дат или правила повторения статусы и отметки отдельных появлений удаляются, потому что прежнее расписание больше не действует. `DELETE /v1/tasks/{id}` возвращает `204`. Статус, приоритет и группа в запросах задаются через `status_id`, `priority_id` и `group_id`; они должны принадлежать текущему пользователю. Даты в запросах и ответах имеют формат `YYYY-MM-DD`.

`GET /v1/task-statuses`, `GET /v1/task-groups` и `GET /v1/task-priorities` возвращают `{"items":[...]}` со справочниками пользователя. Записи отсортированы по `sort_order`, затем по `id`.

Для каждого справочника доступны также `POST /v1/{resource}`, `GET /v1/{resource}/{id}`, `PATCH /v1/{resource}/{id}` и `DELETE /v1/{resource}/{id}`, где `resource` — `task-statuses`, `task-groups` или `task-priorities`. При создании нужны `label`, а для группы ещё `color`. Изменять можно название и цвет, у пользовательского статуса также `is_completed`; порядок присваивается автоматически. Встроенные статусы и приоритеты можно переименовать, но удаление возвращает `409`. Удаление пользовательского статуса переводит связанные задачи и статусы повторений в `status-todo`; удаление приоритета или группы очищает связь у задач.

## Проверка в Docker и Postman

Запустите сервисы из корня бекенда:

```sh
cp .env.example .env
docker compose up --build -d
docker compose ps
curl -i http://localhost:8081/health/ready
```

В Postman выберите **Import** и файл `postman/plans-tracker-local.postman_collection.json`. Переменная коллекции `baseUrl` уже равна `http://localhost:8081`; запустите коллекцию целиком через **Run collection**. Запросы идут в нужном порядке и сохраняют ID созданных записей для следующих шагов. Даты для списка и тестовой задачи вычисляются при запуске. Коллекция создаёт и удаляет собственные записи, встроенные значения только проверяет на защиту от удаления.

Если readiness не возвращает `200`, посмотрите логи:

```sh
docker compose logs --tail=100 postgres tasks redis
```

Для базы, созданной до появления новых миграций, примените команду из раздела «Схема PostgreSQL» и затем запустите dev seed. Это сохранит существующие данные.

## Тесты tasks

`go test ./...` запускает HTTP-тесты с подставным репозиторием. Они проверяют чтение и запись задач, вложенные статус, приоритет и группу, списки справочников, пустые ответы, порядок элементов, передачу пользователя и диапазона дат, валидацию и ответы при ошибках.

Интеграционный тест SQL работает только с отдельной базой `tasks_test`. Создайте её и примените схему один раз после запуска PostgreSQL:

```sh
docker compose up -d postgres
docker compose exec -T postgres createdb -U app tasks_test
docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app -d tasks_test < docker/postgres/migrations/tasks/001_initial.sql
docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app -d tasks_test < docker/postgres/migrations/tasks/002_drop_group_color_check.sql
docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app -d tasks_test < docker/postgres/migrations/tasks/003_occurrence_status_date_index.sql
```

Затем запустите тест, подставив локальный пароль PostgreSQL при необходимости:

```sh
TEST_DATABASE_URL='postgres://app:localdev@localhost:5433/tasks_test?sslmode=disable' go test ./internal/tasks/postgres -v
```

Тесты создают записи двух случайных пользователей, проверяют фильтр по пользователю, пересечение диапазонов дат, CRUD задач и справочников, связи задач со статусами, приоритетами и группами, защиту встроенных значений и справочники, затем удаляют свои данные. Без `TEST_DATABASE_URL` SQL-тесты пропускаются; URL должен указывать именно на `tasks_test`, чтобы не затронуть обычную базу `tasks`.

## Структура

- `cmd/tasks`, `cmd/habits` — отдельные исполняемые сервисы.
- `internal/platform` — конфигурация, подключение зависимостей и HTTP-сервер.
- `docker/postgres/init.sql` — создание баз и применение SQL-миграций.
- `docker/postgres/migrations` — версии схем для каждой базы.
- `openapi.yaml` — текущий HTTP-контракт.

Мобильное приложение сейчас хранит данные локально и пока не подключено к этому API. Ручки привычек ещё не реализованы.
