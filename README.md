# trip-service

Сервис поездок TripGo для курса «Разработка микросервисов на Go».
Все лабораторные работы ведутся в этом репозитории. Сейчас подготовлен каркас лабораторной работы 1.

Материалы курса: [course-go-autumn-2026/course](https://github.com/course-go-autumn-2026/course).

## Требования

- Go 1.24 или новее
- Docker
- [`tripgoctl`](https://github.com/course-go-autumn-2026/course-infra) в `PATH`
- команды ниже рассчитаны на Linux, в том числе Ubuntu в WSL2

## Окружение

Файл `environment.toml` в корне описывает окружение лабораторной работы 1: только PostgreSQL.

```bash
tripgoctl cluster start
tripgoctl environment start
tripgoctl connect
```

`environment start` создаёт `.env` с адресами. Этот файл и каталог `.tripgo/` в git не попадают. Хосты и порты берутся из `.env`.

## Команды

```bash
make generate   # типы и chi-server из contracts/openapi/trip-service.openapi.yaml
make migrate    # накатить миграции goose
make run        # запустить сервис
make test       # go test -race ./...
```

Откат миграций: `make migrate-down`.

## Переменные окружения

Имена зафиксированы курсом. Полный список с безопасными значениями по умолчанию лежит в `.env.example`. Для лабораторной работы 1 нужны:

| Переменная | Назначение |
|---|---|
| `HTTP_ADDR` | адрес HTTP-сервера |
| `LOG_LEVEL` | уровень логов |
| `SHUTDOWN_TIMEOUT` | бюджет graceful shutdown |
| `DATABASE_URL` | строка подключения к PostgreSQL |
| `DATABASE_MAX_CONNS` | верхняя граница пула |
| `DATABASE_MIN_CONNS` | нижняя граница пула |
| `DATABASE_MAX_CONN_LIFETIME` | время жизни соединения |
| `DATABASE_CONNECT_TIMEOUT` | таймаут подключения |
| `DATABASE_QUERY_TIMEOUT` | таймаут запроса |

Рабочий `DATABASE_URL` после `tripgoctl environment start` лежит в `.env`, не в `.env.example`.

## Структура

```text
cmd/trip-service/   точка входа
internal/           код сервиса
migrations/         миграции goose
api/                код, сгенерированный из OpenAPI
contracts/          контракты курса, не редактировать
deploy/             Dockerfile и манифесты, когда понадобятся
```

## Что сделано в этой подготовке

Собран каркас репозитория, скопированы контракты и `environment.toml`, подключены `oapi-codegen` и `goose`.

HTTP-ручки, миграции таблиц и менеджер транзакций ещё не написаны. `cmd/trip-service/main.go` пока только сообщает, что сервер не поднят.

## Решения

Уровень изоляции, устройство менеджера транзакций и запрет двух активных поездок будут описаны здесь, когда появится реализация.
