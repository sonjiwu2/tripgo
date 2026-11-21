# trip-service

Сервис поездок TripGo: создание, получение и завершение поездки, приём координат и опрос устройств, которые давно ничего не присылали. Код разложен по слоям, есть тесты, линтер, логи, метрики и трейсы.

Из заданий со звёздочкой сделал мутационное тестирование.

## Требования

- Go 1.25.4+
- Docker
- [tripgoctl](https://github.com/course-go-autumn-2026/course-infra)
- golangci-lint v2

На Windows всё запускается из WSL2.

## Запуск

```bash
tripgoctl cluster start
tripgoctl environment start
make migrate
make run
```

`tripgoctl environment start` поднимает PostgreSQL, коллектор и заглушку пушей и создаёт `.env`. Ссылки на Jaeger и Grafana показывает `tripgoctl connect`. Логи пушей: `tripgoctl environment logs push-service`.

Проверить, что сервис поднялся: `GET /health` и `GET /ready`.

## Команды

| Команда | Что делает |
|---|---|
| `make run` | запустить сервис |
| `make migrate` | накатить миграции |
| `make migrate-down` | откатить миграцию |
| `make generate` | сгенерировать `api/api.gen.go` и моки |
| `make test` | unit-тесты с `-race` и покрытие |
| `make test-integration` | тесты репозитория на PostgreSQL |
| `make lint` | запустить `golangci-lint` |

## Переменные окружения

Все переменные обязательные: если какой-то нет, сервис не стартует и пишет, какой именно. Пример — в `.env.example`.

**Сервер:** `HTTP_ADDR`, `LOG_LEVEL`, `SHUTDOWN_TIMEOUT`, `HTTP_READ_TIMEOUT`, `HTTP_READ_HEADER_TIMEOUT`, `HTTP_WRITE_TIMEOUT`, `HTTP_IDLE_TIMEOUT`

**База:** `DATABASE_URL`, `DATABASE_MAX_CONNS`, `DATABASE_MIN_CONNS`, `DATABASE_MAX_CONN_LIFETIME`, `DATABASE_CONNECT_TIMEOUT`, `DATABASE_QUERY_TIMEOUT`

**Телеметрия:** `OTEL_SERVICE_NAME`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_PROTOCOL`, `OTEL_RESOURCE_ATTRIBUTES`, `OTEL_TRACES_SAMPLER`

**Координаты и пуш:** `POSITION_CHECK_INTERVAL`, `POSITION_STALE_AFTER`, `POSITION_WORKERS`, `POSITION_QUEUE_SIZE`, `PUSH_HTTP_URL`, `PUSH_REQUEST_TIMEOUT`

## Архитектура

Зависимости идут внутрь: хендлеры знают про usecase, usecase знает про домен, домен не знает ни про HTTP, ни про базу.

```text
cmd/trip-service/          запуск и остановка сервиса
internal/config/           чтение и проверка env
internal/container/        создание и связывание всех частей
internal/domain/trip/      поездка, правила и ошибки
internal/model/            данные новой поездки
internal/usecase/trip/      создание, получение, завершение
internal/usecase/position/ сохранение координаты и маршрут
internal/http/trip/         ручки поездок
internal/http/position/     ручки координат
internal/http/domainerror/ ошибка → HTTP-ответ
internal/http/middleware/  паника, трейсы, логи, метрики
internal/http/router/      сборка ручек и middleware
internal/http/probe/       /health и /ready
internal/http/respond/     запись ответа
internal/http/present/     поездка в формате API
internal/repository/trip/      запросы поездок
internal/repository/position/  координаты и выборка для опроса
internal/gateway/push/         HTTP-клиент Push Service
internal/worker/positions/     сканер, очередь и воркеры
internal/infra/postgres/       пул и транзакции
internal/infra/telemetry/      OpenTelemetry и логгер
api/                       код, сгенерированный из OpenAPI
migrations/                миграции goose
```

`cmd/trip-service` и `api/` названы как в соглашениях курса. Пакеты `router`, `probe`, `respond` и `present` вынес отдельно, чтобы не повторять этот код в каждой ручке.

Интерфейсы лежат в `contract.go` у того пакета, который их использует: так он сам описывает, что ему нужно, а в тестах вместо базы можно подставить мок. Второй транспорт будет вызывать те же usecase.

## Решения

- Ошибки домена переводятся в HTTP только в `domainerror`. При 500 клиент получает общий текст, а подробности идут в лог.
- Создание поездки пишет в `trips` и `trip_status_history` в одной транзакции.
- Одну активную поездку на водителя держит индекс `trips_one_active_driver_idx`.
- Если коллектор недоступен, сервис продолжает работать.

## Опрос устаревших поездок

Пока поездка активна, водитель присылает координаты на `POST /api/v1/trips/{tripId}/positions`. Маршрут целиком отдаёт `GET /api/v1/trips/{tripId}/positions`, точки идут по `recorded_at`, потом по `id`.

Координата пишется в `trip_positions`, а `trips.last_position_at` обновляется в той же транзакции. Время сравнивается в SQL через `GREATEST`: старая точка, которая доехала позже, маршрут дополняет, но поездку не состаривает.

Если координат давно нет, их не копит горутина на каждую поездку. Схема такая:

```text
сканер (один тикер) -> очередь ограниченного размера -> воркеры фиксированного числа -> Push Service
```

Сканер раз в `POSITION_CHECK_INTERVAL` одним запросом берёт активные поездки, у которых `COALESCE(last_position_at, started_at)` старше `POSITION_STALE_AFTER`. Первыми в выборке те, кто молчит дольше. Под запрос есть частичный индекс `trips_active_quiet_idx`.

Очередь — канал на `POSITION_QUEUE_SIZE`. Воркеров `POSITION_WORKERS`, они живут всё время работы сервиса. Поездка не стоит в очереди дважды: id держится в общей map, её проверяет сканер и чистит воркер после обработки. Если координата пришла раньше, хендлер убирает id из map, и воркер в Push не идёт.

Если канал полон, сканер задачу не ждёт: она считается `dropped`, пишется warn, счётчики `tripgo.stale_checks` и `tripgo.position_requests` увеличиваются. Следующий тик попробует снова. Повторный пуш по той же поездке возможен не чаще, чем раз в `POSITION_CHECK_INTERVAL`, потому что отметку об отправке отдельно не храним.

Вызов Push идёт через `internal/gateway/push`, своим `http.Client` и таймаутом `PUSH_REQUEST_TIMEOUT`. Одна попытка, без ретраев. Спан воркера связан со спаном тика ссылкой, а не как ребёнок: обработка живёт отдельно от тика.

Остановка: HTTP перестаёт принимать запросы, потом останавливается сканер, воркеры доделывают уже лежащие в очереди задачи в пределах `SHUTDOWN_TIMEOUT`, затем закрываются клиент пуша и пул базы.

Звёздочки этой работы (выборка батчами и приоритетная очередь) не делал.

## Мутационное тестирование

Гонял `gremlins` v0.6.0 по `internal/domain` и `internal/usecase` со всеми видами мутантов. Он не убирает строки и не подменяет значения, поэтому ещё 17 мутантов сделал руками.

| | Всего | Убито | Выжило | Не покрыто |
|---|---|---|---|---|
| `gremlins`, до | 39 | 37 | 0 | 2 |
| `gremlins`, после | 39 | 39 | 0 | 0 |
| Ручные, до | 17 | 11 | 6 | 0 |
| Ручные, после | 17 | 17 | 0 | 0 |

Что нашлось:

| Проблема | Мутанты | Что сделал |
|---|---|---|
| не покрыт `FieldError.Error` | 2 не покрыто | `TestFieldErrorText` |
| `New`: не проверялись координаты | 4 выжило | проверка в `TestNew` |
| `New`: не проверялся `UpdatedAt` | 1 выжил | проверка в `TestNew` |
| `create`: не проверялись координаты | 1 выжил | проверка в `TestCreate` |
