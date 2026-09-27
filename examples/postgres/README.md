# PostgreSQL example

Runs `gohan` against PostgreSQL through `database/sql` and the pgx stdlib driver: an `INSERT` of
several structs with `RETURNING`, an upsert, a `CASE` aggregate using `gohan.Int`, a jsonb
key-exists filter written with `Raw("meta ?? ?", ...)`, `ILIKE`, `UPDATE ... RETURNING` and a
`DELETE` with an `IN` list.

The program creates a table named `gohan_example_tasks`, and drops it when it finishes.

## Run

Point `GOHAN_PG_DSN` at a database you can create tables in. For a throwaway server:

```
docker run -d --rm --name gohan-pg -p 127.0.0.1:5432:5432 \
  -e POSTGRES_USER=gohan -e POSTGRES_PASSWORD=gohan -e POSTGRES_DB=gohan postgres:17

cd examples
GOHAN_PG_DSN='postgres://gohan:gohan@127.0.0.1:5432/gohan?sslmode=disable' go run ./postgres

docker stop gohan-pg
```

The server may need a few seconds after `docker run` before it accepts connections.
