# Dialects

- [At a glance](#at-a-glance)
- [Selecting a dialect](#selecting-a-dialect)
- [Features and limits](#features-and-limits)
- [PostgreSQL](#postgresql)
- [SQLite](#sqlite)
- [ClickHouse](#clickhouse)
- [Generic](#generic)

## At a glance

A dialect decides identifier quoting, the placeholder style, and which clauses are allowed. The
builder API is the same for all of them.

| | PostgreSQL | SQLite | ClickHouse | Generic |
|---|---|---|---|---|
| Constructor | `Postgres()` | `SQLite()` | `ClickHouse()` | `Generic()` |
| Placeholders | `$1`, `$2`, ... | `?` | `?` | `?` |
| Identifier quoting | `"..."` | `` `...` `` | `"..."`, restricted characters | `"..."` |
| `RETURNING` | yes | yes | no | no |
| `ON CONFLICT` upsert | yes | yes | no | no |
| `INSERT ... DEFAULT VALUES` | yes | yes (not with `ON CONFLICT`) | no | yes |
| `UPDATE` | yes | yes | no (not built) | yes |
| `ILIKE` | yes | no | yes | no |
| `UNION` renders as | `UNION` | `UNION` | `UNION DISTINCT` | `UNION` |
| `FINAL`, `SAMPLE`, `ARRAY JOIN`, `PREWHERE`, `SETTINGS` | no | no | yes | no |
| Bound-argument limit | 65535 | 32766 | none | 999 |

The same query in each dialect:

```go
q := gohan.Select("id", "name").
	From("users").
	Where(gohan.Col("name").Eq("alice"), gohan.Col("age").Gte(18)).
	Limit(1)

for _, d := range []gohan.Dialect{gohan.Postgres(), gohan.SQLite(), gohan.ClickHouse(), gohan.Generic()} {
	sql, args, err := q.Build(d)
	fmt.Println(d.Name()+":", sql, args, err)
}
// Output:
// postgres: SELECT "id", "name" FROM "users" WHERE ("name" = $1 AND "age" >= $2) LIMIT 1 [alice 18] <nil>
// sqlite: SELECT `id`, `name` FROM `users` WHERE (`name` = ? AND `age` >= ?) LIMIT 1 [alice 18] <nil>
// clickhouse: SELECT "id", "name" FROM "users" WHERE ("name" = ? AND "age" >= ?) LIMIT 1 [alice 18] <nil>
// generic: SELECT "id", "name" FROM "users" WHERE ("name" = ? AND "age" >= ?) LIMIT 1 [alice 18] <nil>
```

## Selecting a dialect

`DialectFor(driverName)` returns the dialect registered for a `database/sql` driver name, so the
dialect can follow the driver chosen in configuration:

| Driver name | Dialect |
|---|---|
| `pgx`, `pgx/v5`, `postgres` | PostgreSQL |
| `sqlite`, `sqlite3` | SQLite |
| `clickhouse` | ClickHouse |

`Register(name, dialect)` adds a name or replaces an existing one. It is safe for concurrent use.

```go
gohan.Register("libsql", gohan.SQLite())

d, err := gohan.DialectFor("libsql")
fmt.Println(d.Name(), err)

_, err = gohan.DialectFor("mysql")
fmt.Println(errors.Is(err, gohan.ErrUnknownDialect))
// Output:
// sqlite <nil>
// true
```

The zero `Dialect{}` is not usable: `Build` fails with `ErrUnknownDialect`.

## Features and limits

`Dialect.Has(feature)` reports optional features, and `MaxArgs()` the bound-argument limit (0 for
none). Past the limit, `Build` fails with `ErrTooManyArgs`.

```go
for _, d := range []gohan.Dialect{gohan.Postgres(), gohan.SQLite(), gohan.ClickHouse(), gohan.Generic()} {
	fmt.Printf("%-10s returning=%-5v upsert=%-5v ilike=%-5v update=%-5v clickhouse=%-5v maxargs=%d\n",
		d.Name(),
		d.Has(gohan.FeatureReturning),
		d.Has(gohan.FeatureUpsert),
		d.Has(gohan.FeatureILike),
		d.Has(gohan.FeatureUpdate),
		d.Has(gohan.FeatureClickHouse),
		d.MaxArgs(),
	)
}
// Output:
// postgres   returning=true  upsert=true  ilike=true  update=true  clickhouse=false maxargs=65535
// sqlite     returning=true  upsert=true  ilike=false update=true  clickhouse=false maxargs=32766
// clickhouse returning=false upsert=false ilike=true  update=false clickhouse=true  maxargs=0
// generic    returning=false upsert=false ilike=false update=true  clickhouse=false maxargs=999
```

Using a clause the dialect does not have fails at `Build` with `ErrUnsupported`.

`FeatureLocking` (row locking — see [Row locking](select.md#row-locking)) is set only on
`Postgres()`; `SQLite()`, `ClickHouse()`, `ClickHouseNamed()` and `Generic()` all report
`d.Has(gohan.FeatureLocking) == false`.

`FeatureDefaultValues` (`INSERT ... DEFAULT VALUES` — see
[DEFAULT VALUES](insert-and-upsert.md#default-values)) is set on `Postgres()`, `SQLite()` and
`Generic()`; `ClickHouse()` and `ClickHouseNamed()` report `d.Has(gohan.FeatureDefaultValues) ==
false`, since real ClickHouse rejects `INSERT INTO t DEFAULT VALUES` with a syntax error.

`Dialect.QuoteIdent(name)` quotes a name for hand-written SQL such as DDL. Dots separate qualified
parts, `*` is kept as is, and quote characters inside a part are escaped:

```go
for _, d := range []gohan.Dialect{gohan.Postgres(), gohan.SQLite(), gohan.ClickHouse()} {
	q, err := d.QuoteIdent(`audit."log"`)
	fmt.Println(d.Name()+":", q, err)
}
// Output:
// postgres: "audit"."""log""" <nil>
// sqlite: `audit`.`"log"` <nil>
// clickhouse: "audit"."""log""" <nil>
```

An empty name, an empty part (`"a..b"`) or a NUL byte fails with `ErrInvalidIdentifier` on every
dialect.

## PostgreSQL

- Placeholders are numbered (`$1`, `$2`, ...) in the order they appear, across subqueries, CTEs
  and `UNION` members.
- A bound value on its own in a select list has no type PostgreSQL can infer, and is sent as
  `text`; wrap it in `Cast` (for example `gohan.Cast(1, "int")`).
- Integer constants inside a `CASE` that feeds an aggregate should use `gohan.Int`: a bound `$n`
  there is typed `text`, and `SUM(text)` does not exist. See [Int](expressions.md#integer-literals-int).
- Each use of an expression with bound arguments gets new placeholders, so the same `Fn(...)` in
  the select list and in `GROUP BY` does not match; group by the output alias instead. See
  [Aggregates and functions](expressions.md#aggregates-and-functions).
- jsonb operators that contain `?` are written with `??` in `Raw`. See
  [Raw](raw-and-escape-hatches.md#raw).

## SQLite

- Identifiers are quoted with backticks. SQLite treats a double-quoted name that matches no column
  as a string literal instead of raising an error; backticks have no such fallback.
- `LIKE` is case-insensitive for ASCII letters, and there is no `ILIKE` (`ErrUnsupported`).
- An `INSERT ... SELECT` with `ON CONFLICT` gets `WHERE true` when the last select has no `WHERE`,
  which SQLite's grammar requires. See
  [INSERT and upsert](insert-and-upsert.md#sqlite-insert--select-with-on-conflict).
- `RETURNING` and `ON CONFLICT` need SQLite 3.35 and 3.24 or later respectively; `gohan` does not
  check the SQLite version.

An `OFFSET` without `LIMIT` is rendered as `LIMIT -1 OFFSET n`, since SQLite has no bare `OFFSET`:

```go
sql, args, err := gohan.From("users").Offset(40).Build(gohan.SQLite())
// sql: SELECT * FROM `users` LIMIT -1 OFFSET 40
// args: []
```

## ClickHouse

### ClickHouse-only clauses

These fail with `ErrUnsupported` on every other dialect.

**`Final()`** reads a `ReplacingMergeTree` (or other collapsing engine) with rows merged:

```go
sql, args, err := gohan.From("events").Final().Where(gohan.Col("tenant_id").Eq(7)).Build(gohan.ClickHouse())
// sql: SELECT * FROM "events" FINAL WHERE "tenant_id" = ?
// args: [7]
```

**`Sample(ratio)`** and **`SampleRows(n)`** read a sample of a table that has a `SAMPLE BY` key.
The ratio must be in (0, 1] (`ErrInvalidSample`):

```go
sql, args, err := gohan.Select(gohan.CountAll()).From("hits").Sample(0.1).Build(gohan.ClickHouse())
// sql: SELECT COUNT(*) FROM "hits" SAMPLE 0.1
// args: []
```

**`ArrayJoin(cols...)`** and **`LeftArrayJoin(cols...)`** unfold arrays into rows. Call one of
them at most once per builder, passing every column in that call:

```go
sql, args, err := gohan.Select("id", "tag").
	From("articles").
	ArrayJoin(gohan.Col("tags").As("tag")).
	Build(gohan.ClickHouse())
// sql: SELECT "id", "tag" FROM "articles" ARRAY JOIN "tags" AS "tag"
// args: []
```

**`Prewhere(conds...)`** filters before the remaining columns are read:

```go
sql, args, err := gohan.Select("url", "duration").
	From("requests").
	Prewhere(gohan.Col("date").Eq("2024-06-01")).
	Where(gohan.Col("duration").Gt(1000)).
	Build(gohan.ClickHouse())
// sql: SELECT "url", "duration" FROM "requests" PREWHERE "date" = ? WHERE "duration" > ?
// args: [2024-06-01 1000]
```

**`Settings(map)`** appends query settings. Keys must match `^[A-Za-z_][A-Za-z0-9_]*$`
(`ErrInvalidSetting`) and are sorted; values are bound. Repeated calls merge:

```go
sql, args, err := gohan.From("events").
	Limit(100).
	Settings(map[string]any{"max_threads": 4, "join_use_nulls": 1}).
	Build(gohan.ClickHouse())
// sql: SELECT * FROM "events" LIMIT 100 SETTINGS join_use_nulls = ?, max_threads = ?
// args: [1 4]
```

The clauses combine in ClickHouse's order:

```go
sql, args, err := gohan.From(gohan.Table("events").As("e")).
	Final().
	Sample(0.5).
	Prewhere(gohan.Col("tenant_id").Eq(7)).
	Where(gohan.Col("kind").Eq("click")).
	Limit(10).
	Settings(map[string]any{"max_threads": 2}).
	Build(gohan.ClickHouse())
// sql: SELECT * FROM "events" AS "e" FINAL SAMPLE 0.5 PREWHERE "tenant_id" = ? WHERE "kind" = ? LIMIT 10 SETTINGS max_threads = ?
// args: [7 click 2]
```

`Final` and `Sample` cannot be applied to a subquery source (`ErrUnsupported`).

### Statements

- The dialect does not build `UPDATE`, `RETURNING` or `ON CONFLICT` (`ErrUnsupported`). This
  includes ClickHouse's lightweight `UPDATE`, available in recent server versions.
- `DELETE` is ClickHouse's lightweight delete. It requires a `WHERE`, so `Delete(t).All()` renders
  `DELETE FROM t WHERE 1`. A table alias in `DELETE` is not supported. Lightweight deletes do not
  work on every engine (for example Distributed tables or tables with projections).
- `Union` renders `UNION DISTINCT`, because ClickHouse rejects a bare `UNION` unless the
  `union_default_mode` setting is set. A compound with an outer `ORDER BY`, `LIMIT` or `OFFSET` is
  wrapped as `SELECT * FROM (...) ORDER BY ...`. See [UNION](select.md#union).

### Identifiers

Identifiers are double-quoted, with `"` doubled and `\` escaped. They may not contain `?`, `@`,
`{`, `}`, or `$` followed by a digit, which clickhouse-go or the server could read as parameter
syntax (`ErrInvalidIdentifier`):

```go
_, _, err := gohan.Select("a?b").From("t").Build(gohan.ClickHouse())
fmt.Println(err)
// Output:
// gohan: invalid identifier: "a?b"
```

### Values

clickhouse-go binds `?` placeholders on the client, by formatting each value into the SQL text. To
keep that safe, `gohan` checks every value bound for ClickHouse, including elements of slices and
arrays:

- maps, structs (other than `time.Time`), and values implementing `error` or `fmt.Formatter` fail
  with `ErrUnsafeValue`, unless they implement `fmt.Stringer`;
- a `driver.Valuer` is exempt from these checks only as a top-level value (clickhouse-go calls
  `Value()` only on those). Nested inside a slice it is checked like any other value, so a
  struct-based `Valuer` such as `sql.NullString` fails. Pass it on its own, or through `In(...)`,
  which binds each element as a top-level value;
- a top-level nil pointer to a `driver.Valuer` type is bound as `NULL` (clickhouse-go would
  otherwise call `Value()` on it and panic);
- a statement builder passed as a value fails with `ErrInvalidColumn`.

```go
_, _, err := gohan.From("t").
	Where(gohan.Col("attrs").Eq(map[string]string{"k": "v"})).
	Build(gohan.ClickHouse())
fmt.Println(errors.Is(err, gohan.ErrUnsafeValue), err)
// Output:
// true gohan: value type cannot be bound safely for this dialect: map[string]string
```

`Contains`, `HasPrefix` and `HasSuffix` escape with backslashes on ClickHouse (there is no `ESCAPE`
clause there):

```go
sql, args, err := gohan.From("products").Where(gohan.Col("name").Contains(`50%_off\`)).Build(gohan.ClickHouse())
// sql: SELECT * FROM "products" WHERE "name" LIKE ?
// args: [%50\%\_off\\%]
```

### LEFT JOIN and NULL

A ClickHouse `LEFT JOIN` fills unmatched columns with the column type's default (`0`, `''`), not
`NULL`, unless the `join_use_nulls` setting is 1. Code that tests a joined column for `NULL` should
set it:

```go
sql, args, err := gohan.Select("u.id", "o.id").
	From(gohan.Table("users").As("u")).
	LeftJoin(gohan.Table("orders").As("o"), gohan.Col("o.user_id").Eq(gohan.Col("u.id"))).
	Where(gohan.Col("o.id").IsNull()).
	Settings(map[string]any{"join_use_nulls": 1}).
	Build(gohan.ClickHouse())
// sql: SELECT "u"."id", "o"."id" FROM "users" AS "u" LEFT JOIN "orders" AS "o" ON "o"."user_id" = "u"."id" WHERE "o"."id" IS NULL SETTINGS join_use_nulls = ?
// args: [1]
```

### Time precision

clickhouse-go formats a bound `time.Time` at **seconds precision** when binding positional `?`
placeholders (measured against clickhouse-go v2.40.3 and v2.48.0), so a `DateTime64(3/6/9)` column
silently loses its sub-second part on insert and in `WHERE` comparisons. This is a property of
clickhouse-go's positional binding, not of `gohan`'s rendering. Use `ClickHouseNamed` below to keep
full precision.

### Named parameters

`ClickHouseNamed()` returns the same dialect as `ClickHouse()` (same identifier quoting, feature
set and argument limit) but renders each bound value as `@p1`, `@p2`, … instead of `?`, and returns
it as a `sql.NamedArg{Name: "p1", Value: v}` instead of the bare value. It is not in the default
driver registry; call `gohan.Register` if you want it looked up by driver name.

This is the only way to keep `DateTime64` sub-second precision through clickhouse-go: the driver's
client-side `@name` binding calls `clickhouse.DateNamed(name, t, scale)` when the caller converts
the argument, which formats the value at the scale you choose instead of truncating it to seconds.
Convert each `time.Time` argument after `Build` and before passing args to `database/sql`:

```go
q, args, err := st.Build(gohan.ClickHouseNamed())
for i, a := range args {
    na := a.(sql.NamedArg)
    if t, ok := na.Value.(time.Time); ok {
        args[i] = clickhouse.DateNamed(na.Name, t, clickhouse.NanoSeconds)
    }
}
rows, err := db.QueryContext(ctx, q, args...)
```

For clickhouse-go's native API (`clickhouse.Conn`, not `database/sql`), convert every
`sql.NamedArg` to `clickhouse.Named(na.Name, na.Value)` instead — the native API's binder
recognizes only its own `driver.NamedValue`/`driver.NamedDateValue` types, not `sql.NamedArg`
(observed: passing a raw `sql.NamedArg` to `conn.Exec` fails with a parse error, since the
placeholder is left unsubstituted).

`Raw` text must not contain `@` followed by a letter, digit or `_` in named mode
(`ErrRawPlaceholder`): clickhouse-go v2.40.3 substitutes such a token even inside a string literal,
so `Raw("'@p1' = x")` is rejected on `ClickHouseNamed()` even though the same text is accepted on
`ClickHouse()`.

Server-side `{name:Type}` query parameters are not supported by this mode (or by `gohan` at all):
their string escaping differs between clickhouse-go versions, they cannot be mixed with `?` in one
statement, and `{...:...}` text breaks positional binding even inside a string literal.

## Generic

`Generic()` is ANSI SQL: double-quoted identifiers, `?` placeholders, `UPDATE`, and none of the
optional features. Use it for databases that follow the standard and are not listed above.

Do not use it for MySQL or MariaDB: there a double-quoted string is a string literal, not an
identifier (unless `ANSI_QUOTES` is enabled), so the SQL would compare against constant strings.
