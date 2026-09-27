# gohan examples

Small end-to-end programs that build statements with `gohan` and run them through `database/sql`.
They live in their own Go module (`github.com/oddbit-project/gohan/examples`), so the drivers they
use are not dependencies of `gohan` itself. The module points at the parent directory with a
`replace` directive, so the examples always build against the local source.

| Example | Database | Needs |
|---|---|---|
| [sqlite](sqlite/) | SQLite, in memory (`modernc.org/sqlite`, no cgo) | nothing |
| [postgres](postgres/) | PostgreSQL (`github.com/jackc/pgx/v5/stdlib`) | a server, `GOHAN_PG_DSN` |
| [clickhouse](clickhouse/) | ClickHouse (`github.com/ClickHouse/clickhouse-go/v2`) | a server, `GOHAN_CH_DSN` |

Run them from this directory:

```
cd examples
go run ./sqlite
```

Each program prints every statement it builds, its arguments, and the result.
See the [documentation](../docs/) for the full API.
