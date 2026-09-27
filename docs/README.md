# gohan documentation

`gohan` is a SQL query builder for Go in which values are always bound and identifiers are always
quoted. It renders a SQL string and an argument list for PostgreSQL, SQLite, ClickHouse or a generic
ANSI dialect. It does not run queries: pass the result to `database/sql` or your driver.

Every SQL string and argument list in these pages is copied from real `gohan` output.

## Guides

| Page | Contents |
|---|---|
| [Getting started](getting-started.md) | Install, build a first query, run it with `database/sql`, pick a dialect, handle errors |
| [SELECT](select.md) | Columns, `WHERE`, `ORDER BY`, `LIMIT`/`OFFSET`, joins, `GROUP BY`/`HAVING`, subqueries, CTEs, `UNION` |
| [INSERT and upsert](insert-and-upsert.md) | `Values`, `Rows`, `SetMap`, `INSERT ... SELECT`, `ON CONFLICT`, `RETURNING` |
| [UPDATE and DELETE](update-and-delete.md) | `Set`, `SetMap`, `SetRecord`, the `WHERE` requirement, `All()`, `RETURNING` |
| [Expressions](expressions.md) | `Col` and `Val`, comparisons, `IN`, `LIKE` helpers, `And`/`Or`/`Not`, aggregates, `Fn`, `Cast`, `Case`, `Int` |
| [Raw SQL and escape hatches](raw-and-escape-hatches.md) | `Raw`, `Fn` and `Cast`, and what is validated in each |
| [Dialects](dialects.md) | What changes per dialect, driver-name lookup, PostgreSQL, SQLite, ClickHouse and Generic notes |
| [Records and struct tags](records-and-struct-tags.md) | Mapping structs to columns, tag reference, `RecordColumns`/`InsertColumns`, the `field` package |
| [Security model](security-model.md) | What `gohan` guarantees, what is trusted input, and what remains your job |
| [Errors](errors.md) | Every `Err*` value and when it is returned |

## Other resources

- [API reference on pkg.go.dev](https://pkg.go.dev/github.com/oddbit-project/gohan): every exported
  symbol, with runnable examples.
- [Runnable examples](../examples/): end-to-end programs for SQLite, PostgreSQL and ClickHouse.
- [Project README](../README.md) and [security policy](../SECURITY.md).
