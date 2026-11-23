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

Обязательные настройки приходят из окружения. Если одна из них пропущена, сервис сообщает, какая именно. Пример настроек в `.env.example`, значения по умолчанию для опроса приведены ниже.

**Сервер:** `HTTP_ADDR`, `LOG_LEVEL`, `SHUTDOWN_TIMEOUT`, `HTTP_READ_TIMEOUT`, `HTTP_READ_HEADER_TIMEOUT`, `HTTP_WRITE_TIMEOUT`, `HTTP_IDLE_TIMEOUT`

**База:** `DATABASE_URL`, `DATABASE_MAX_CONNS`, `DATABASE_MIN_CONNS`, `DATABASE_MAX_CONN_LIFETIME`, `DATABASE_CONNECT_TIMEOUT`, `DATABASE_QUERY_TIMEOUT`

**Телеметрия:** `OTEL_SERVICE_NAME`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_PROTOCOL`, `OTEL_RESOURCE_ATTRIBUTES`, `OTEL_TRACES_SAMPLER`

**Координаты и пуш:** `POSITION_CHECK_INTERVAL`, `POSITION_STALE_AFTER`, `POSITION_WORKERS`, `POSITION_QUEUE_SIZE`, `POSITION_SCAN_BATCH_SIZE`, `PUSH_HTTP_URL`, `PUSH_REQUEST_TIMEOUT`

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

`POST /api/v1/trips/{tripId}/positions` сохраняет координату активной поездки. `GET /api/v1/trips/{tripId}/positions` отдаёт весь маршрут по времени `recorded_at`, при равном времени по `id`.

Координата и `trips.last_position_at` пишутся в одной транзакции через `TxManager`. В запросе `GREATEST` оставляет самое позднее время, даже если устройство потом прислало более старую точку.

```text
сканер → ограниченная приоритетная очередь → воркеры → Push Service
```

| Переменная | По умолчанию | Для чего нужна |
|---|---|---|
| `POSITION_CHECK_INTERVAL` | `30s` | период сканера |
| `POSITION_STALE_AFTER` | `60s` | сколько поездка должна молчать, чтобы попасть в выборку |
| `POSITION_SCAN_BATCH_SIZE` | `100` | максимальное число строк в одном запросе сканера |
| `POSITION_WORKERS` | `4` | число воркеров |
| `POSITION_QUEUE_SIZE` | `100` | число ожидающих задач |
| `PUSH_HTTP_URL` | из `.env` от `tripgoctl` | адрес Push Service |
| `PUSH_REQUEST_TIMEOUT` | `1s` | таймаут отправки |
| `SHUTDOWN_TIMEOUT` | `10s` в `.env.example` | общий бюджет остановки сервиса |

На каждом тике SQL выбирает активные поездки по `COALESCE(last_position_at, started_at)` и `id`. Следующая порция начинается после последней строки предыдущей. Под оба запроса подходит частичный индекс `trips_active_quiet_cursor_idx`. Гистограмма `tripgo.stale_checks.rows_read` показывает, сколько строк прочитал сканер за тик.

Воркер берёт самую давно молчащую поездку из всей очереди. В очереди и в работе по одной задаче на поездку. Приход координаты убирает ожидающую задачу, а метрика `tripgo.position_requests` получает `result=cancelled`.

При полной очереди более старая задача вытесняет самую свежую. Если новой задаче самой не хватает приоритета, отбрасывается она. Любой дроп пишет Warn и увеличивает `tripgo.stale_checks{result="dropped"}`. Сканер на первом дропе завершает текущий обход. Следующий тик начинает выборку с начала. Если координата не пришла, поездка снова может попасть в выборку на следующем тике.

Воркер делает одну HTTP-попытку со своим таймаутом. Спан задачи связан со спаном сканера через span link. Вызов Push Service виден отдельным спаном.

При остановке HTTP заканчивает текущие запросы, сканер завершает работу, воркеры разбирают очередь в пределах общего таймаута. Затем закрываются телеметрия, HTTP-клиент и пул PostgreSQL.

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
