# gohan documentation

`gohan` is a SQL query builder for Go in which values are always bound and identifiers are always
quoted. It renders a SQL string and an argument list for PostgreSQL, SQLite, ClickHouse or a generic
ANSI dialect. It does not run queries: pass the result to `database/sql` or your driver.

Every SQL string and argument list in these pages is copied from real `gohan` output.

## Why "gohan"?

gohan is a spiritual successor to [goqu](https://github.com/doug-martin/goqu), the expressive Go
SQL builder. The name is a nod to that lineage: *goqu* reads like *Goku*, and in Dragon Ball,
Gohan is Goku's son — the next generation, raised on what came before, carrying it forward in
its own way. (*Gohan* is also the Japanese word for a meal of rice, which suits a library you use
every day.)

**What it inherits from goqu**

- A fluent builder: `Select`, `From`, `Where`, `Join`, `OrderBy`, `Insert`, `Update`, `Delete`,
  `OnConflict`, `Returning`, CTEs and `UNION` read the way goqu users expect.
- Dialect awareness: one query definition renders for PostgreSQL, SQLite, ClickHouse or generic
  ANSI SQL.
- Map conditions (`Match(map)`, in the spirit of `goqu.Ex`) and struct-based inserts and updates.
- goqu's struct-tag vocabulary: `goqu:"skipinsert"`, `goqu:"skipupdate"`, `goqu:"omitnil"` and
  `goqu:"omitempty"` are honoured, so existing record types keep working.

**What it does differently**

- **Values are always bound.** There is no interpolated mode that writes values into the SQL
  text, so there is no mode in which a forgotten setting turns user input into SQL.
- **Identifiers are always quoted and escaped**, with per-dialect rules (including ClickHouse's
  backslash escapes and SQLite's backtick quoting).
- **Refuses dangerous or ambiguous SQL at build time**: an `UPDATE` or `DELETE` without a `WHERE`
  needs an explicit `All()`, and ClickHouse values the driver would format unsafely are rejected.
- **Builds, never runs.** `Build(dialect)` returns SQL and arguments; execution belongs to
  `database/sql` or your driver.
- **Immutable builders**: every method returns a copy, so a base query can be shared and extended
  safely.

gohan is an independent project, written from scratch; it is not a fork of goqu and is not
affiliated with its authors.

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
- [Runnable examples](https://github.com/oddbit-project/gohan/tree/main/examples): end-to-end programs for SQLite, PostgreSQL and ClickHouse.
- [Project README](https://github.com/oddbit-project/gohan/blob/main/README.md) and [security policy](https://github.com/oddbit-project/gohan/blob/main/SECURITY.md).
