# Room Booking Service

Реализована обязательная часть тестового задания на Go.

## Что использовано

- Go 1.26
- PostgreSQL
- `chi` для HTTP-роутинга
- `pgx` как PostgreSQL-драйвер
- `golang-jwt` для JWT

## Запуск

```bash
docker-compose up --build
```

Сервис поднимается на `localhost:8080`.

## Тесты

```bash
go test ./... -cover
```

## Makefile

- `make up` - поднять проект через Docker Compose
- `make down` - остановить проект и удалить тома
- `make build` - собрать приложение
- `make test` - запустить тесты
- `make seed` - наполнить сервис тестовыми данными через HTTP API
- `make swagger` - сгенерировать Swagger-документацию из аннотаций
- `make lint` - запустить `golangci-lint`

## Swagger

- Swagger UI доступен по адресу `http://localhost:8080/swagger/index.html`
- Документация генерируется из аннотаций в коде командой `make swagger`
- Сгенерированные файлы лежат в каталоге `docs`

## Структура проекта

- `cmd/app` - точка входа и сборка зависимостей приложения.
- `internal/domain` - доменные модели и ошибки.
- `internal/httpapi` - HTTP-роуты, middleware и JSON-обвязка.
- `internal/service` - бизнес-логика.
- `internal/db/postgres` - SQL-слой, миграция схемы и реализация репозитория.
- `internal/testsupport` - in-memory хранилище для тестов.

## Зависимости между слоями

- `domain` ни от кого не зависит.
- `service` зависит от `domain` и интерфейса репозитория.
- `httpapi` зависит от `service`, `domain` и JWT-парсинга.
- `db/postgres` зависит от `domain` и реализует контракт репозитория для `service`.
- `cmd/app` только связывает всё вместе через dependency injection.

## Как работать с проектом

1. Запускать проект через `docker-compose up --build`.
2. Проверять код командой `go test ./... -cover`.
3. Для ручной проверки сначала получать токен через `POST /dummyLogin`.
4. Если меняется бизнес-правило, править `internal/service`.
5. Если меняется SQL или хранение данных, править `internal/db/postgres`.
6. Если меняется HTTP-контракт, править `internal/httpapi`.
7. Если меняется сборка или конфигурация, править `cmd/app` и `internal/config`.

## Базовый сценарий ручной проверки

1. `POST /dummyLogin` с `{"role":"admin"}`.
2. `POST /rooms/create`.
3. `POST /rooms/{roomId}/schedule/create`.
4. `POST /dummyLogin` с `{"role":"user"}`.
5. `GET /rooms/{roomId}/slots/list?date=YYYY-MM-DD`.
6. `POST /bookings/create`.
7. `POST /bookings/{bookingId}/cancel`.

## Что реализовано

- `GET /_info`
- `POST /register`
- `POST /login`
- `POST /dummyLogin`
- `GET /rooms/list`
- `POST /rooms/create`
- `POST /rooms/{roomId}/schedule/create`
- `GET /rooms/{roomId}/slots/list`
- `POST /bookings/create`
- `GET /bookings/list`
- `GET /bookings/my`
- `POST /bookings/{bookingId}/cancel`

## Принятые решения

- Дополнительно реализованы `/register` и `/login` из списка optional-задач.
- Пароль при регистрации хранится в базе не в открытом виде, а в виде salted hash.
- Слоты создаются по запросу для конкретной переговорки и даты через `/rooms/{roomId}/slots/list`, после чего сохраняются в базе. За счёт этого у слотов стабильные UUID в БД, а повторные запросы используют те же записи.
- Список доступных слотов возвращает только будущие свободные слоты на указанную дату. Прошедшие интервалы не считаются доступными для бронирования.
- Время расписания принимается в UTC в формате `HH:MM`. Для корректного разбиения на 30-минутные слоты начало и конец расписания должны быть кратны 30 минутам.

## Примечания по проверке

- Для ролей `admin` и `user` в `/dummyLogin` используются фиксированные UUID, как требует задание.
- Отмена брони идемпотентна: повторный вызов возвращает `200` и актуальное состояние брони.
- В `/bookings/my` возвращаются только брони на будущие слоты.
