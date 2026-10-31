# Plans Tracker backend

Go-бекенд для мобильного приложения. Сервисы `accounts`, `tasks`, `habits` и `calendar` запускаются отдельно, используют отдельные базы данных в одном PostgreSQL и общий Redis. В сервисе `tasks` есть CRUD задач, статусов, приоритетов и групп. SQL-схемы лежат в `docker/postgres/migrations`.

## Запуск

Нужны Docker и Docker Compose. Из этой папки:

```sh
cp .env.example .env
# Укажите в .env путь к .p8 и задайте два случайных секрета.
docker compose up --build -d
curl -i http://localhost:8081/health/ready
curl -i http://localhost:8082/health/ready
curl -i http://localhost:8083/health/ready
curl -i http://localhost:8084/health/ready
```

Обычный локальный запуск автоматически подключает `compose.override.yaml`: при первом создании PostgreSQL он добавляет тестового пользователя и четыре задачи из `docker/postgres/seed.dev.sql`. Продакшен-команда ниже явно указывает Compose-файлы и не подключает dev seed.

Тестовый пользователь: `8dc53fe1-0f08-4bd0-a3ed-6a5541859490` (`dev@example.invalid`). Локальный Compose разрешает запросы без токена для этого `DEV_USER_ID`. Если локальная база была создана раньше, добавьте записи вручную из корня проекта:

```sh
docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app -d postgres < docker/postgres/seed.dev.sql
```

Seed можно запускать повторно: существующие записи он не дублирует. Прежде чем запускать его на старой базе, примените SQL-схему командой из раздела «Схема PostgreSQL».

Сервисы доступны на `localhost:8081`, `localhost:8082`, `localhost:8083` и `localhost:8084`. PostgreSQL доступен локально на порту `5433`, Redis на `6380`. Все опубликованные порты привязаны только к `127.0.0.1`. Локально PostgreSQL хранит данные в слое контейнера, Redis не сохраняет их на диск. `docker compose down` удаляет контейнер и локальную базу. Для очистки также созданных образом анонимных томов используйте `docker compose down -v`.

## Хранение данных в продакшене

Файл `compose.prod.yaml` добавляет постоянный том PostgreSQL. Задайте отдельный `PROD_POSTGRES_PASSWORD` в окружении или в `.env` и запускайте с обоими файлами:

```sh
docker compose -p plans-prod -f compose.yaml -f compose.prod.yaml up --build -d
docker compose -p plans-prod -f compose.yaml -f compose.prod.yaml down
```

Том сохраняется после перезапуска или пересоздания контейнера и после обычного `down`. Не применяйте к продакшену `down -v`: эта команда удалит том с базой. Том не заменяет резервные копии и не защищает от потери диска сервера. Redis в обеих конфигурациях остаётся временным хранилищем.

`.env.example` содержит пароль только для локального запуска. Перед использованием вне локальной машины задайте свой `PROD_POSTGRES_PASSWORD` и настройте доступ к Redis. В продакшене пароль передаётся приложению через `DATABASE_PASSWORD`, отдельно от URL. Продакшен-конфигурация не включает `AUTH_ALLOW_DEV_USER`.

## Схема PostgreSQL

`docker/postgres/init.sql` создаёт базы `accounts`, `tasks`, `habits`, `calendar` и применяет SQL-файлы из `docker/postgres/migrations` при первом запуске PostgreSQL. Повторный запуск скрипта пропускает уже применённые версии. Если том PostgreSQL уже существует, запустите скрипт вручную после запуска контейнера, чтобы применить новые миграции:

```sh
docker compose exec -T postgres psql -U app -d postgres -f /docker-entrypoint-initdb.d/01-init.sql
```

Для продакшен-стека добавьте к команде `-p plans-prod -f compose.yaml -f compose.prod.yaml` перед `exec`.

Таблицы соответствуют текущим локальным моделям приложения:

- `accounts`: `users`, настройки приложения, оформления и уведомлений; прежняя таблица настроек календаря сохранена для переноса;
- `tasks`: статусы, приоритеты, группы, задачи, статусы повторений, отметки выполнения; прежняя таблица событий календаря сохранена для переноса;
- `habits`: категории, привычки, отметки выполнения;
- `calendar`: события и настройки календаря.

В базе `habits` правило повторения обязательно: `daily`, `weekly` с непустым списком уникальных дней недели от 0 (воскресенье) до 6, `monthly` с днём от 1 до 31 или `custom` с положительным интервалом дней. Категория необязательна. При её удалении привычки сохраняются, `category_id` становится `NULL`, а `updated_at` обновляется; удаление привычки удаляет её отметки выполнения. `updated_at` обновляется и при любом другом изменении привычки. Повторения вычисляются приложением от даты создания привычки, а отметка хранит отдельно календарный день `occurrence_date` и фактическое время `completed_at`.

У каждого пользователя свои статусы и приоритеты. При создании первой задачи базовые значения появляются автоматически. Чтобы получить их до первой задачи, вызовите в базе `tasks` функцию `SELECT ensure_task_defaults('<user-uuid>');`. Ссылки `user_id` из разных баз на `accounts.users` не имеют внешнего ключа, поскольку базы раздельны; удаление связанных данных координирует сервис `accounts`. Ссылки событий календаря на привычки также проверяются бекендом. Apple-идентификаторы и сессии хранятся отдельно от `users.email`. Значение `notification_settings.enabled` отражает пользовательскую настройку, а фактическое разрешение ОС остаётся на устройстве.

## Локальный запуск Go

После запуска зависимостей через `docker compose up -d postgres redis`:

```sh
DATABASE_URL='postgres://app:localdev@localhost:5433/tasks?sslmode=disable' REDIS_ADDR='localhost:6380' DEV_USER_ID='8dc53fe1-0f08-4bd0-a3ed-6a5541859490' AUTH_ALLOW_DEV_USER=true PORT=8081 go run ./cmd/tasks
DATABASE_URL='postgres://app:localdev@localhost:5433/habits?sslmode=disable' REDIS_ADDR='localhost:6380' DEV_USER_ID='8dc53fe1-0f08-4bd0-a3ed-6a5541859490' AUTH_ALLOW_DEV_USER=true PORT=8082 go run ./cmd/habits
DATABASE_URL='postgres://app:localdev@localhost:5433/calendar?sslmode=disable' REDIS_ADDR='localhost:6380' TASKS_URL='http://localhost:8081' HABITS_URL='http://localhost:8082' DEV_USER_ID='8dc53fe1-0f08-4bd0-a3ed-6a5541859490' AUTH_ALLOW_DEV_USER=true PORT=8083 go run ./cmd/calendar
```

Обязательны `DATABASE_URL` и `REDIS_ADDR`; опциональны `DATABASE_PASSWORD`, `REDIS_PASSWORD` и `PORT` (по умолчанию `8080`). Для защищённых запросов задайте `AUTH_INTROSPECT_URL` и `AUTH_INTERNAL_SECRET` либо включите `AUTH_ALLOW_DEV_USER=true` только локально. `GET /health/live` проверяет процесс, `GET /health/ready` проверяет PostgreSQL и Redis. Контракт описан в `openapi.yaml`.

## Авторизация через Apple

Сервис `accounts` работает на `127.0.0.1:8084`. В `.env` укажите `APPLE_TEAM_ID`, `APPLE_KEY_ID`, абсолютный `APPLE_PRIVATE_KEY_PATH`, `APPLE_TOKEN_ENCRYPTION_KEY` (base64 от 32 случайных байт) и `AUTH_INTERNAL_SECRET` (минимум 32 случайных символа). `APPLE_CLIENT_ID` в Compose равен iOS Bundle ID `app.planstracker`. Файл `.p8` подключается в контейнер только для чтения, его содержимое не записывается в репозиторий. Пример генерации секретов: `openssl rand -base64 32` и `openssl rand -hex 32`. Не меняйте ключ шифрования после создания аккаунтов без переноса зашифрованных Apple refresh token.

Порядок входа для iOS-клиента:

1. `POST /v1/auth/challenges` возвращает одноразовый `nonce`, действительный 5 минут. Передайте его в нативный запрос Sign in with Apple.
2. Передайте `identityToken`, `authorizationCode`, тот же `nonce` и имя пользователя при первом согласии в `POST /v1/auth/apple` как `identity_token`, `authorization_code`, `nonce`, `display_name`. Backend проверяет подпись и поля токена по ключам Apple, обменивает код и возвращает свою сессию.
3. Передавайте `access_token` как `Authorization: Bearer <token>` в `tasks`, `habits`, `calendar`. Он действует 15 минут. `POST /v1/auth/refresh` с `refresh_token` возвращает новую пару токенов; предыдущая пара сразу перестаёт действовать. Refresh token действует 30 дней и должен храниться в защищённом хранилище устройства.
4. `GET /v1/auth/me` возвращает текущего пользователя, `POST /v1/auth/logout` отзывает текущую сессию, `DELETE /v1/auth/account` удаляет данные пользователя из всех баз и отзывает Apple refresh token. При временной ошибке удаления ответ `503`; повторите запрос с ещё действующей сессией.

Email может быть скрытым или отсутствовать; аккаунт связывается по проверенному Apple `sub`. Локальные данные мобильного приложения автоматически не переносятся в новый аккаунт. В обычном локальном Compose `AUTH_ALLOW_DEV_USER=true` допускает запросы без токена для тестового пользователя; при переданном некорректном Bearer-токене всё равно возвращается `401`. В продакшен-стеке обхода нет.

Для быстрой реакции на отзыв согласия и удаление Apple Account настройте в Apple Developer серверные уведомления на публичный HTTPS-адрес `https://ваш-домен/v1/auth/apple/notifications`. Сервис проверяет подпись JWS, отзывает сессии при `consent-revoked`, удаляет аккаунт при `account-deleted` и учитывает изменение пересылки email. Повторное уведомление с тем же `jti` обрабатывается безопасно. Без публичного HTTPS-адреса этот канал нельзя включить в Apple Developer; при обновлении собственной сессии backend дополнительно проверяет Apple refresh token не чаще одного раза в сутки.

## Сервис calendar

Сервис календаря доступен на `localhost:8083`. Он хранит события и настройки в отдельной базе `calendar`. Пользователь определяется по токену; локальный Compose разрешает запросы без токена для `DEV_USER_ID`. Ссылки события на задачу или привычку проверяются по `TASKS_URL` и `HABITS_URL`; недоступность связанного сервиса даёт `503`, отсутствующая у пользователя запись — `400`.

- `GET /v1/calendar-events?start_date=YYYY-MM-DD&end_date=YYYY-MM-DD` возвращает события, у которых есть хотя бы одно не исключённое появление в диапазоне до 366 дней. Повторяющееся событие возвращается один раз.
- `POST /v1/calendar-events`, `GET /v1/calendar-events/{id}`, `PUT /v1/calendar-events/{id}` и `DELETE /v1/calendar-events/{id}` создают, читают, полностью заменяют и удаляют событие.
- `PUT /v1/calendar-events/{id}/excluded-dates/{date}` исключает один запланированный день, `DELETE` по тому же пути возвращает его.
- `GET /v1/calendar-settings` и `PATCH /v1/calendar-settings` читают и меняют `show_tasks`, `show_habits`, `view_mode`.

Форма JSON описана в `openapi.yaml`. Для локальной проверки добавлена коллекция `postman/collections/calendar`. Расписание задач и привычек по-прежнему читается через их собственные API; сервис календаря хранит только пользовательские планы и ссылки на них.

Старые таблицы `tasks.calendar_events` и `accounts.calendar_settings` не удаляются автоматически. Если они уже содержат нужные данные, перенесите их перед переключением клиентов на новый API. Для этого можно выгрузить строки `pg_dump --data-only --table=calendar_events` из БД `tasks` в новую БД `calendar`, а `calendar_settings` — из `accounts`; сначала сделайте резервную копию и убедитесь, что целевые таблицы пусты.

## Каркас сервиса tasks

`cmd/tasks/main.go` соединяет слои. `internal/platform` открывает PostgreSQL, Redis и HTTP-сервер, затем вызывает регистрацию маршрутов сервиса. Код задач расположен отдельно:

- `internal/tasks/task.go` — начальная доменная модель. Дополняйте её по мере реализации функций.
- `internal/tasks/service.go` — интерфейс хранилища и место для правил работы с задачами.
- `internal/tasks/postgres/repository.go` — чтение и запись задач и справочников; запросы для отдельных появлений задачи лежат в `occurrence.go`.
- `internal/tasks/httpapi/handler.go` — REST-маршруты и обработчики.
- `internal/tasks/httpapi/dto.go` — место для структур JSON-запросов и ответов.

`GET /v1/tasks?start_date=2026-10-10&end_date=2026-10-20` возвращает `{"items":[...]}` для текущего пользователя (в локальном Compose — для `DEV_USER_ID`). Обе даты обязательны, формат — `YYYY-MM-DD`, максимум 366 дней. В список входят задачи хотя бы с одним запланированным днём в диапазоне; повторяющаяся задача возвращается один раз, даже если началась раньше. Каждый элемент содержит вложенный `status`; `priority` и `group` содержат объекты или `null`, если не заданы. `GET /v1/tasks/backlog` возвращает задачи без дат и повторения. Схема задачи в мобильном приложении находится в `../plans-tracker-mobile/src/shared/api/tasks/model.ts`.

`GET /v1/task-occurrences?start_date=2026-10-10&end_date=2026-10-20` возвращает отдельный элемент для каждого дня задачи. У элемента есть `task`, `occurrence_date`, `status`, `completed`, `completed_at`. Повторения `daily`, `weekly` (`daysOfWeek`: 0 — воскресенье), `monthly` (`dayOfMonth`) и `custom` (`everyNDays`) считаются от `start_date`, а если её нет — от даты создания задачи по UTC. Исходный `end_date` не ограничивает будущие повторения; для неповторяющейся задачи учитывается весь диапазон от `start_date` до `end_date`. Несуществующие даты месяца пропускаются.

`GET /v1/tasks/{id}/occurrences/{date}` читает одно появление. Для повторяющейся задачи `PUT /v1/tasks/{id}/occurrences/{date}/status` с `{"status_id":"status-done"}` задаёт статус только на эту дату и создаёт отметку выполнения; любой статус с `is_completed=true` работает так же. `PUT /v1/tasks/{id}/occurrences/{date}/completion` с `{"completed":false}` снимает отметку и сбрасывает завершённый статус на `status-todo`. Для неповторяющейся задачи меняйте общий статус через `PATCH /v1/tasks/{id}`; запись статуса отдельного появления вернёт `409`.

`GET /v1/tasks/{id}` возвращает одну задачу в том же формате, включая задачи без дат. Для отсутствующей или чужой задачи возвращается `404`.

`POST /v1/tasks` создаёт задачу и возвращает `201` с заголовком `Location`. Обязателен `title`; по умолчанию используются `status-todo` и 60 минут. Если передана только одна из дат, она копируется в другую. `PATCH /v1/tasks/{id}` изменяет только переданные поля; чтобы изменить или очистить диапазон дат, передайте обе даты. При изменении дат или правила повторения статусы и отметки отдельных появлений удаляются, потому что прежнее расписание больше не действует. `DELETE /v1/tasks/{id}` возвращает `204`. Статус, приоритет и группа в запросах задаются через `status_id`, `priority_id` и `group_id`; они должны принадлежать текущему пользователю. Даты в запросах и ответах имеют формат `YYYY-MM-DD`.

`GET /v1/task-statuses`, `GET /v1/task-groups` и `GET /v1/task-priorities` возвращают `{"items":[...]}` со справочниками пользователя. Записи отсортированы по `sort_order`, затем по `id`.

Сервис `habits` работает на порту `8082` для текущего пользователя (в локальном Compose — для `DEV_USER_ID`). `GET /v1/habits` возвращает `{"items":[...]}` со всеми привычками пользователя, отсортированными по времени создания (новые первыми), затем по ID. Пустой список возвращается как `{"items":[]}`. В ответе есть `category_id`, обязательное `repeat_rule`, время начала и длительность; отметки выполнения загружаются отдельно.

Для привычек доступны `POST /v1/habits`, `GET /v1/habits/{id}`, `PATCH /v1/habits/{id}` и `DELETE /v1/habits/{id}`. Создание требует `title` и `repeat_rule`; по умолчанию заметка и описание пустые, длительность — 60 минут, категория и время начала отсутствуют. PATCH меняет только переданные поля; `null` очищает `category_id` или `start_minute`. Удаление привычки удаляет её отметки. Чужая или отсутствующая привычка возвращает `404`.

Категории доступны через `GET` и `POST /v1/habit-categories`, а также `GET`, `PATCH` и `DELETE /v1/habit-categories/{id}`. При создании нужен `label`; порядок назначается автоматически. Удаление категории оставляет связанные привычки без категории.

`GET /v1/habit-completions` возвращает отметки всех привычек пользователя одной выборкой. Можно передать `habit_id` и пару `start_date`/`end_date` в формате `YYYY-MM-DD`; диапазон включает обе даты и ограничен 366 днями. Без дат возвращается полная история для расчёта стриков в приложении. `PUT /v1/habits/{id}/completions/{date}` с `{"completed":true}` создаёт или обновляет отметку, с `{"completed":false}` снимает её; ответ содержит текущее состояние и объект отметки либо `null`. Дата отметки не обязана совпадать с расписанием, как и в локальном репозитории приложения. Недельная полоска, статистика и стрики по-прежнему вычисляются в приложении из привычек и отметок.

Для каждого справочника доступны также `POST /v1/{resource}`, `GET /v1/{resource}/{id}`, `PATCH /v1/{resource}/{id}` и `DELETE /v1/{resource}/{id}`, где `resource` — `task-statuses`, `task-groups` или `task-priorities`. При создании нужны `label`, а для группы ещё `color`. Изменять можно название и цвет, у пользовательского статуса также `is_completed`; порядок присваивается автоматически. Встроенные статусы и приоритеты можно переименовать, но удаление возвращает `409`. Удаление пользовательского статуса переводит связанные задачи и статусы повторений в `status-todo`; удаление приоритета или группы очищает связь у задач.

## Проверка в Docker и Postman

Запустите сервисы из корня бекенда:

```sh
cp .env.example .env
# Укажите в .env путь к .p8 и задайте два случайных секрета.
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

Для `habits` HTTP-тесты проверяют список, создание, частичное изменение, отметки, диапазон дат, валидацию и ошибки. SQL-тесты запускаются отдельно с `HABITS_TEST_DATABASE_URL`; они проверяют CRUD, порядок, связи, отметки и изоляцию пользователей. URL должен указывать только на отдельную базу `habits_test`. После запуска PostgreSQL подготовьте её так:

```sh
docker compose exec -T postgres createdb -U app habits_test
docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app -d habits_test < docker/postgres/migrations/habits/001_initial.sql
docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app -d habits_test < docker/postgres/migrations/habits/002_category_delete_and_repeat_rules.sql
HABITS_TEST_DATABASE_URL='postgres://app:localdev@localhost:5433/habits_test?sslmode=disable' go test ./internal/habits/postgres -v
```

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
- `internal/habits` — модель, правила ввода, PostgreSQL-репозиторий и HTTP-обработчики привычек.
- `docker/postgres/init.sql` — создание баз и применение SQL-миграций.
- `docker/postgres/migrations` — версии схем для каждой базы.
- `openapi.yaml` — текущий HTTP-контракт.

Мобильное приложение сейчас хранит данные локально и пока не подключено к этому API.
