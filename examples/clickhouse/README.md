# ClickHouse example

Runs `gohan` against ClickHouse through `database/sql` and clickhouse-go v2: a multi-row `INSERT`
into a `ReplacingMergeTree` table, then `SELECT`s using `FINAL`, `PREWHERE`, `SETTINGS`,
`ARRAY JOIN` and `SAMPLE`, a lightweight `DELETE`, and two statements `gohan` refuses on
ClickHouse (an `UPDATE`, and a map value that clickhouse-go cannot bind safely).

The program creates a table named `gohan_example_events`, and drops it when it finishes.

## Run

Point `GOHAN_CH_DSN` at a database you can create tables in, using the native protocol port. For a
throwaway server:

```
docker run -d --rm --name gohan-ch -p 127.0.0.1:9000:9000 \
  -e CLICKHOUSE_USER=gohan -e CLICKHOUSE_PASSWORD=gohan -e CLICKHOUSE_DB=gohan \
  --ulimit nofile=262144:262144 clickhouse/clickhouse-server

cd examples
GOHAN_CH_DSN='clickhouse://gohan:gohan@127.0.0.1:9000/gohan' go run ./clickhouse

docker stop gohan-ch
```

The server may need a few seconds after `docker run` before it accepts connections.
