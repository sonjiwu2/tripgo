# trip-service

Сервис TripGo создаёт и завершает поездки, принимает координаты и отдаёт маршрут. Если от водителя давно нет координат, сервис запрашивает их через Push Service.

## Локальный запуск

Нужны Go 1.25.4+, Docker и [tripgoctl](https://github.com/course-go-autumn-2026/course-infra). Для линтера нужен golangci-lint v2. На Windows команды запускаются из WSL2.

```bash
tripgoctl cluster start
tripgoctl environment start
tripgoctl connect
make migrate
make run
```

`tripgoctl environment start` поднимает PostgreSQL, коллектор телеметрии и Push Service, и создаёт `.env`. `make migrate` и `make run` читают настройки из этого файла. `tripgoctl connect` показывает адреса сервисов, Grafana и Jaeger.

Проверить запуск можно через `GET /health` и `GET /ready`. Логи Push Service доступны через `tripgoctl environment logs push-service`.

## Проверки

```bash
make test
make test-integration
make lint
```

`make test` запускает unit-тесты с `-race` и печатает покрытие домена и usecase. Для `make test-integration` нужен PostgreSQL из окружения. `make generate` обновляет OpenAPI-код и моки, `make migrate-down` откатывает последнюю миграцию.

## Архитектура

Хендлеры вызывают usecase, а правила и ошибки поездки лежат в домене. Интерфейсы объявлены в `contract.go` у пакетов, которые ими пользуются. Реализации собираются в `internal/container`.

| Где | Что лежит |
|---|---|
| `cmd/trip-service`, `internal/container` | запуск, остановка и сборка зависимостей |
| `internal/domain/trip`, `internal/model`, `internal/usecase` | правила поездок, модели и сценарии |
| `internal/http`, `api` | ручки, middleware и сгенерированный HTTP-контракт |
| `internal/repository`, `internal/infra`, `migrations` | запросы к PostgreSQL, транзакции, телеметрия и схема базы |
| `internal/worker/positions`, `internal/gateway/push` | опрос поездок и клиент Push Service |

Пакеты `router`, `probe`, `respond` и `present` содержат общий код HTTP-ручек. Доменные ошибки переводятся в HTTP-ответы в `domainerror`. При 500 подробности попадают в лог. Создание поездки и запись в историю статусов идут в одной транзакции. Индекс `trips_one_active_driver_idx` разрешает одну активную поездку на водителя.

## Опрос устаревших поездок

`POST /api/v1/trips/{tripId}/positions` сохраняет координату активной поездки. Точка и `last_position_at` записываются в одной транзакции через `TxManager`. Если старая точка пришла позже новой, `GREATEST` сохраняет более позднее время. `GET /api/v1/trips/{tripId}/positions` отдаёт все точки по `recorded_at`, при одинаковом времени — по `id`.

```text
сканер → приоритетная очередь → воркеры → Push Service
```

На каждом тике сканер выбирает активные поездки, по которым давно нет координат. Если точек ещё не было, отсчёт идёт от `started_at`. Поездки читаются порциями от самых старых к свежим. Для следующей порции используется ключ `(время, id)` последней строки. Запрос использует индекс `trips_active_quiet_cursor_idx`.

Воркер берёт самую старую задачу из очереди и один раз отправляет `REQUEST_POSITION` по HTTP. У поездки может быть только одна задача в очереди или в работе. Если приходит координата, ожидающая задача удаляется и учитывается как `cancelled`. Если координата так и не пришла, поездку можно снова выбрать на следующем тике.

При полной очереди более старая задача вытесняет самую свежую. Если новая задача не старше неё, отбрасывается новая. В обоих случаях пишется `Warn` и растёт `tripgo.stale_checks{result="dropped"}`. На первом дропе сканер заканчивает текущий обход, а на следующем тике начинает выборку с начала.

При остановке сервис сначала заканчивает HTTP-запросы, затем останавливает сканер. Воркеры разбирают очередь в пределах `SHUTDOWN_TIMEOUT`, после чего закрываются телеметрия, HTTP-клиент и пул базы.

### Настройки опроса

| Переменная | Значение по умолчанию | Назначение |
|---|---|---|
| `POSITION_CHECK_INTERVAL` | `30s` | период сканера |
| `POSITION_STALE_AFTER` | `60s` | через сколько без координат поездка попадает в выборку |
| `POSITION_SCAN_BATCH_SIZE` | `100` | размер одной порции из базы |
| `POSITION_WORKERS` | `4` | число воркеров |
| `POSITION_QUEUE_SIZE` | `100` | сколько задач может ждать в очереди |
| `PUSH_HTTP_URL` | из `.env` | адрес Push Service |
| `PUSH_REQUEST_TIMEOUT` | `1s` | таймаут запроса в Push Service |
| `SHUTDOWN_TIMEOUT` | `10s` в `.env.example` | общий бюджет остановки |

Число прочитанных за тик строк видно в `tripgo.stale_checks.rows_read`. В Grafana можно посмотреть размер очереди, число занятых воркеров и результаты запросов. В трассировке есть отдельный спан Push Service, а спан задачи связан с тиком сканера через span link.

### Остальные настройки

Полный пример есть в [`.env.example`](.env.example).

- Сервер — `HTTP_ADDR`, `LOG_LEVEL`, `HTTP_READ_TIMEOUT`, `HTTP_READ_HEADER_TIMEOUT`, `HTTP_WRITE_TIMEOUT`, `HTTP_IDLE_TIMEOUT`.
- База — `DATABASE_URL`, `DATABASE_MAX_CONNS`, `DATABASE_MIN_CONNS`, `DATABASE_MAX_CONN_LIFETIME`, `DATABASE_CONNECT_TIMEOUT`, `DATABASE_QUERY_TIMEOUT`.
- Телеметрия — `OTEL_SERVICE_NAME`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_PROTOCOL`, `OTEL_RESOURCE_ATTRIBUTES`, `OTEL_TRACES_SAMPLER`.

## Мутационное тестирование (ЛР2)

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
