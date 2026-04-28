# Board of Directors Backend

Backend API для проекта **Board of Directors** — игры, в которой пользователи создают партии, отправляют действия, а состояние восстанавливается из истории событий.

Проект написан на Go и использует PostgreSQL для хранения данных. API предоставляет CRUD-операции для пользователей и игр, а игровая часть реализована в event-sourced стиле: клиент отправляет действия, а backend отвечает персонализированным состоянием партии.

## Возможности

- Создание, получение, обновление и удаление пользователей
- Создание, получение, обновление и удаление игр
- Event-sourced хранение истории партии
- Отправка игровых действий и получение персонализированного состояния игры
- Автоматический запуск SQL-миграций при старте приложения
- Swagger UI / OpenAPI-спецификация
- Запуск через Docker Compose

## Технологии

- Go
- PostgreSQL
- Docker / Docker Compose
- chi router
- pgx PostgreSQL driver
- OpenAPI / Swagger UI

## Структура проекта

```text
.
├── cmd/
│   └── main.go              # Точка входа приложения
├── internal/
│   ├── config/              # Загрузка конфигурации из переменных окружения
│   ├── httpserver/          # HTTP-роутер и обработчики API
│   ├── models/              # Доменные модели: User, Game, Event
│   └── storage/             # Интерфейс хранилища и PostgreSQL-реализация
├── migrations/              # SQL-миграции базы данных
├── Dockerfile
├── docker-compose.yml
└── README.md
```

## Сущности

### User

Пользователь игры.

```json
{
  "id": 1,
  "name": "Alice",
  "created_at": "2026-04-28T12:00:00Z"
}
```

### Game

Игровая сессия.

```json
{
  "id": 1,
  "title": "Board of Directors: Game 1",
  "created_at": "2026-04-28T12:00:00Z"
}
```

### Event

Событие внутри игры.

```json
{
  "id": 1,
  "game_id": 1,
  "user_id": 1,
  "actor_name": "Alice",
  "event_type": "player_joined",
  "event_value": "",
  "created_at": "2026-04-28T12:00:00Z"
}
```

## Типы событий

В проекте предусмотрены следующие типы игровых событий:

```text
game_created
player_joined
player_kicked
game_started
mole_selected
mole_targets_generated
player_received_share
ceo_selected
voting_round_started
vote_submitted
voting_resolved
decision_accepted
decision_rejected
game_finished
```

## Переменные окружения

Приложение использует следующие переменные окружения:

| Переменная | Описание | Значение по умолчанию |
|---|---|---|
| `PORT` | Порт HTTP-сервера | `8080` |
| `POSTGRES_DSN` | Строка подключения к PostgreSQL | обязательная |

Пример:

```env
PORT=8000
POSTGRES_DSN=postgres://agent:pass@localhost:5432/board-of-directors?sslmode=disable
```

## Запуск через Docker Compose

Самый простой способ запустить проект:

```bash
docker compose up --build
```

После запуска приложение будет доступно по адресу:

```text
http://localhost:8000
```

PostgreSQL будет доступен на порту:

```text
localhost:5432
```

Параметры базы данных из `docker-compose.yml`:

```text
database: board-of-directors
user: agent
password: pass
```

## Swagger / OpenAPI

OpenAPI-спецификация доступна по адресу:

```text
http://localhost:8000/openapi.yaml
```

Swagger UI доступен по корневому адресу:

```text
http://localhost:8000/
```

## API

### Users

#### Создать пользователя

```http
POST /users/
Content-Type: application/json
```

```json
{
  "name": "Alice"
}
```

#### Получить список пользователей

```http
GET /users/
```

#### Получить пользователя по ID

```http
GET /users/{id}
```

#### Обновить пользователя

```http
PUT /users/{id}
Content-Type: application/json
```

```json
{
  "name": "Alice Updated"
}
```

#### Удалить пользователя

```http
DELETE /users/{id}
```

---

### Games

#### Создать игру

```http
POST /games/
Content-Type: application/json
```

```json
{
  "title": "Game 1",
  "host_user_id": 1
}
```

При создании игры backend транзакционно пишет события `game_created` и `player_joined` для хоста.

#### Получить список игр

```http
GET /games/
```

#### Получить игру по ID

```http
GET /games/{id}
```

#### Обновить игру

```http
PUT /games/{id}
Content-Type: application/json
```

```json
{
  "title": "Updated Game Title"
}
```

#### Удалить игру

```http
DELETE /games/{id}
```

#### Получить состояние игры

```http
GET /games/{id}/state?viewer_user_id=1
```

Возвращает персонализированное состояние партии. Цели крота раскрываются только самому кроту.

#### Отправить действие игрока

```http
POST /games/{id}/actions
Content-Type: application/json
```

```json
{
  "user_id": 1,
  "type": "vote",
  "payload": {
    "decision": "A"
  }
}
```

MVP-действия:

```text
join_game
kick_player
start_game
vote
```

## Миграции

При старте приложение автоматически выполняет все `.sql`-файлы из директории `migrations`.

Текущая схема создает таблицы:

- `users`
- `games`
- `events`

А также индексы для быстрого поиска событий по:

- `game_id`
- `user_id`
- `event_type`

## Локальный запуск без Docker

Для запуска без Docker нужен установленный PostgreSQL.

1. Создать базу данных:

```bash
createdb board-of-directors
```

2. Указать переменные окружения:

```bash
export PORT=8000
export POSTGRES_DSN="postgres://agent:pass@localhost:5432/board-of-directors?sslmode=disable"
```

3. Запустить backend:

```bash
go run ./cmd/server
```

## Запуск frontend

Frontend запускается отдельным Vite dev server и использует временную dev-идентификацию через `user_id` / `viewer_user_id`.

```bash
cd frontend
npm install
npm run dev
```

По умолчанию frontend ходит в backend по:

```text
VITE_API_BASE_URL=http://localhost:8000
```

Пример файла окружения лежит в `frontend/.env.example`.

## Примеры curl-запросов

### Создание пользователя

```bash
curl -X POST http://localhost:8000/users/ \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice"}'
```

### Создание игры

```bash
curl -X POST http://localhost:8000/games/ \
  -H "Content-Type: application/json" \
  -d '{"title":"First Game","host_user_id":1}'
```

### Получение состояния партии

```bash
curl "http://localhost:8000/games/1/state?viewer_user_id=1"
```

### Отправка действия

```bash
curl -X POST http://localhost:8000/games/1/actions \
  -H "Content-Type: application/json" \
  -d '{"user_id":2,"type":"join_game"}'
```

## Dev frontend

Frontend предназначен для ручного тестирования партии:

- выбор текущего пользователя через список users
- создание игры через `POST /games/`
- загрузка состояния через `GET /games/{id}/state?viewer_user_id=...`
- отправка действий через `POST /games/{id}/actions`

Текущая схема временная и используется только для dev-режима.

TODO:

- позже заменить `user_id` / `viewer_user_id` на session middleware

## Примечания для разработки

Сейчас проект уже содержит базовую игровую механику на event sourcing. Уже есть:

- REST API
- доменный пакет `internal/game`
- модели данных
- слой хранения
- PostgreSQL
- миграции
- Docker Compose
- Swagger/OpenAPI
- персонализированный `GET state`
- endpoint для действий игроков

## Возможное улучшение миграции

В модели `Event.UserID` поле nullable:

```go
UserID *int64 `json:"user_id,omitempty"`
```

Поле `user_id` в SQL-миграции должно быть nullable, иначе `ON DELETE SET NULL` работать не сможет:

```sql
user_id BIGINT REFERENCES users(id) ON DELETE SET NULL
```

Так событие сможет остаться в истории даже после удаления пользователя.