# trip-service

Сервис поездок TripGo: создание, получение и завершение поездки по контракту `contracts/openapi/trip-service.openapi.yaml`, данные хранятся в PostgreSQL.

Лабораторная работа 1 сделана целиком, задания со звёздочкой не делал.

## Требования

- Go 1.25.4+
- Docker
- [tripgoctl](https://github.com/course-go-autumn-2026/course-infra)

На Windows всё запускается из WSL2.

## Запуск

```bash
tripgoctl cluster start
tripgoctl environment start
make migrate
make run
```

`tripgoctl environment start` поднимает PostgreSQL и создаёт `.env`.

Таймауты HTTP-сервера в стандартный набор переменных не входят, поэтому они заданы в `environment.toml` и попадают в `.env` вместе с остальными:

```toml
[env]
HTTP_READ_TIMEOUT = "5s"
HTTP_READ_HEADER_TIMEOUT = "5s"
HTTP_WRITE_TIMEOUT = "10s"
HTTP_IDLE_TIMEOUT = "60s"
```

Проверить, что сервис поднялся: `GET /health` и `GET /ready`.

## Команды

| Команда | Что делает |
|---|---|
| `make run` | запустить сервис |
| `make migrate` | накатить миграции |
| `make migrate-down` | откатить миграцию |
| `make generate` | сгенерировать `api/api.gen.go` из OpenAPI |
| `make test` | тесты с `-race` (в этой работе тестов нет) |

## Переменные окружения

Все переменные обязательные: если какой-то нет, сервис не стартует и пишет, какой именно. Пример — в `.env.example`.

**Сервер:** `HTTP_ADDR`, `LOG_LEVEL`, `SHUTDOWN_TIMEOUT`, `HTTP_READ_TIMEOUT`, `HTTP_READ_HEADER_TIMEOUT`, `HTTP_WRITE_TIMEOUT`, `HTTP_IDLE_TIMEOUT`

**База:** `DATABASE_URL`, `DATABASE_MAX_CONNS`, `DATABASE_MIN_CONNS`, `DATABASE_MAX_CONN_LIFETIME`, `DATABASE_CONNECT_TIMEOUT`, `DATABASE_QUERY_TIMEOUT`

## Структура

```text
cmd/trip-service/   запуск: конфиг, пул, HTTP-сервер, graceful shutdown
internal/config/    чтение и проверка env
internal/trip/      модель поездки и доменные ошибки
internal/postgres/  пул, менеджер транзакций, репозиторий
internal/httpapi/   хендлеры, валидация, ошибки в problem+json
api/                код, сгенерированный из OpenAPI
migrations/         миграции goose
```

## Решения

### Уровень изоляции

Используется `READ COMMITTED`, он задаётся при старте транзакции в `Do`.

Правила, которые могут сломаться при параллельных запросах, держит сама база: одну активную поездку на водителя — уникальный индекс, повторное завершение — условие в `UPDATE`. Более строгий уровень ничего бы тут не дал, а ошибки сериализации пришлось бы ретраить.

### Менеджер транзакций

`Do` открывает транзакцию, кладёт её в контекст и вызывает функцию. Если функция вернула ошибку или упала с паникой — rollback, иначе commit. Вложенный `Do` находит транзакцию в контексте и новую не открывает.

```go
func (m *manager) Do(ctx context.Context, fn func(ctx context.Context) error) error {
    if _, ok := getTransaction(ctx); ok {
        return fn(ctx)
    }

    tx, err := m.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
    if err != nil {
        return fmt.Errorf("не удалось начать транзакцию: %w", err)
    }

    committed := false
    defer func() {
        if !committed {
            _ = tx.Rollback(ctx)
        }
    }()

    err = fn(context.WithValue(ctx, txKey{}, tx))
    if err != nil {
        return err
    }

    err = tx.Commit(ctx)
    if err != nil {
        return fmt.Errorf("не удалось зафиксировать транзакцию: %w", err)
    }
    committed = true
    return nil
}
```

Репозиторий берёт транзакцию из контекста, а если её там нет, работает через пул, так что аргументом в методы она не передаётся:

```go
func (r *Repo) save(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
    ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
    defer cancel()
    if tx, ok := getTransaction(ctx); ok {
        return tx.Exec(ctx, query, args...)
    }
    return r.pool.Exec(ctx, query, args...)
}
```

Модель и ошибки в `internal/trip` от pgx не зависят, SQL и транзакции — в `internal/postgres`. Создание поездки пишет в `trips` и `trip_status_history` в одном `Do`.

### Одна активная поездка на водителя

Частичный уникальный индекс в миграции:

```sql
CREATE UNIQUE INDEX trips_one_active_driver_idx
    ON trips (driver_id)
    WHERE status = 'active';
```

При одновременном создании вторая вставка получает от PostgreSQL ошибку `23505`. Репозиторий по коду и имени индекса превращает её в `ErrDriverBusy`, и ручка отвечает `409 driver_busy`:

```go
return pgErr.Code == "23505" && pgErr.ConstraintName == "trips_one_active_driver_idx"
```

### Завершение поездки

Статус проверяется прямо в `UPDATE`, поэтому между проверкой и записью нет окна для второго запроса:

```go
query, args, err := sqlb.Update("trips").
    Set("status", "completed").
    Set("finished_at", moment).
    Set("updated_at", moment).
    Where("id = ? AND status = ?", id, "active").
    ToSql()
```

Если обновилось 0 строк, поездка дочитывается, и ответ — `404 trip_not_found` или `409 trip_completed`:

```go
if tag.RowsAffected() == 0 {
    found, readErr := r.Get(ctx, id)
    if readErr != nil {
        return trip.Trip{}, readErr
    }
    if found.Status == "completed" {
        return trip.Trip{}, trip.ErrCompleted
    }
    return trip.Trip{}, fmt.Errorf("поездка нашлась, но закрыть её не вышло")
}
```
