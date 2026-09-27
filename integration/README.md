# integration

A separate Go module (`integration/go.mod`) that runs gohan's rendered SQL against real
PostgreSQL, SQLite and ClickHouse servers through `database/sql`. Unit tests in the root module
compare rendered strings; these tests prove the databases accept that SQL and return the intended
result. gohan's own `go.mod` gains no dependency from this module.

## Run it

```
make integration
```

This starts PostgreSQL and ClickHouse in Docker, waits for both to answer their health checks,
runs the suite with `GOHAN_REQUIRE_ENGINES=postgres,sqlite,clickhouse` (so a missing engine fails
loudly instead of being silently skipped), and stops the containers afterwards even if the tests
fail.

## Run it manually

```
docker run -d --rm --name gohan-pg -p 55432:5432 \
	-e POSTGRES_USER=gohan -e POSTGRES_PASSWORD=gohan -e POSTGRES_DB=gohan \
	postgres:17
docker run -d --rm --name gohan-ch -p 59000:9000 -p 58123:8123 \
	-e CLICKHOUSE_USER=gohan -e CLICKHOUSE_PASSWORD=gohan -e CLICKHOUSE_DB=gohan \
	clickhouse/clickhouse-server:26.7

cd integration
GOHAN_PG_DSN='postgres://gohan:gohan@localhost:55432/gohan?sslmode=disable' \
GOHAN_CH_DSN='clickhouse://gohan:gohan@localhost:59000/gohan' \
GOHAN_REQUIRE_ENGINES=postgres,sqlite,clickhouse \
go test -race -count=1 -v ./...

docker stop gohan-pg gohan-ch
```

SQLite always runs (a fresh in-memory database per test); PostgreSQL and ClickHouse run only if
their DSN environment variable is set, unless named in `GOHAN_REQUIRE_ENGINES`, in which case a
missing DSN fails the test instead of skipping it.

The credentials above are throwaway values for local containers only.
