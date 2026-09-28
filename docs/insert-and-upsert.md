# INSERT and upsert

- [Columns and values](#columns-and-values)
- [From structs: Rows](#from-structs-rows)
- [From a map: SetMap](#from-a-map-setmap)
- [INSERT ... SELECT](#insert--select)
- [DEFAULT VALUES](#default-values)
- [ON CONFLICT (upsert)](#on-conflict-upsert)
- [RETURNING](#returning)
- [Errors](#errors)

`Insert(table)` takes a table name (`"schema.table"` allowed) or a `gohan.Table(name)` without an
alias. Exactly one row source is used: `Values`, `Rows`, `SetMap`, `FromSelect` or `DefaultValues`;
combining them fails with `ErrInsertMixed`. Every value that is not an expression is bound.

## Columns and values

`Columns` sets the column list, and each `Values` call appends one row:

```go
sql, args, err := gohan.Insert("users").
	Columns("name", "email").
	Values("alice", "alice@example.com").
	Values("bob", "bob@example.com").
	Build(gohan.Postgres())
// sql: INSERT INTO "users" ("name", "email") VALUES ($1, $2), ($3, $4)
// args: [alice alice@example.com bob bob@example.com]
```

A value that is an expression is rendered in place instead of bound, which is how to write a SQL
default or function call:

```go
sql, args, err := gohan.Insert("events").
	Columns("kind", "created_at").
	Values("login", gohan.Fn("now")).
	Build(gohan.Postgres())
// sql: INSERT INTO "events" ("kind", "created_at") VALUES ($1, now())
// args: [login]
```

A slice is bound as a single value (for array columns), not expanded:

```go
sql, args, err := gohan.Insert("articles").
	Columns("title", "tags").
	Values("hello", []string{"go", "sql"}).
	Build(gohan.Postgres())
// sql: INSERT INTO "articles" ("title", "tags") VALUES ($1, $2)
// args: [hello [go sql]]
```

Each row must have exactly as many values as there are columns (`ErrValueCount`). Large batches
are limited by the dialect's bound-argument limit (65535 on PostgreSQL, 32766 on SQLite, 999 on
Generic); past it `Build` fails with `ErrTooManyArgs`, so split the batch.

## From structs: Rows

`Rows(records...)` takes structs or pointers to structs of one type. Columns come from struct tags
(see [Records and struct tags](records-and-struct-tags.md)); fields tagged `auto` (for example a
serial id) are skipped:

```go
type User struct {
	ID       int64   `db:"id" auto:"true"`
	Name     string  `db:"name"`
	Email    string  `db:"email"`
	Nickname *string `db:"nickname" goqu:"omitnil"`
}

sql, args, err := gohan.Insert("users").
	Rows(
		User{Name: "alice", Email: "alice@example.com"},
		&User{Name: "bob", Email: "bob@example.com"},
	).
	Build(gohan.Postgres())
// sql: INSERT INTO "users" ("name", "email") VALUES ($1, $2), ($3, $4)
// args: [alice alice@example.com bob bob@example.com]
```

The first record decides the column list. A nil `goqu:"omitnil"` pointer or a zero
`goqu:"omitempty"` field is left out; if a later record disagrees (for example its `Nickname` is
set), `Build` fails with `ErrInconsistentOmit` instead of silently binding the wrong columns:

```go
type User struct {
	Name     string  `db:"name"`
	Nickname *string `db:"nickname" goqu:"omitnil"`
}
nick := "bobby"

_, _, err := gohan.Insert("users").
	Rows(User{Name: "alice"}, User{Name: "bob", Nickname: &nick}).
	Build(gohan.Postgres())
fmt.Println(errors.Is(err, gohan.ErrInconsistentOmit))
// Output:
// true
```

Records of different types fail with `ErrRecordType`. Values are copied when `Rows` is called;
changing a record's fields afterwards does not change the statement (the copy is shallow: slice
and map fields still share their contents).

## From a map: SetMap

`SetMap` inserts one row from a map; columns are sorted by key so the output is deterministic:

```go
sql, args, err := gohan.Insert("settings").
	SetMap(map[string]any{"key": "theme", "value": "dark"}).
	Build(gohan.Postgres())
// sql: INSERT INTO "settings" ("key", "value") VALUES ($1, $2)
// args: [theme dark]
```

## INSERT ... SELECT

`FromSelect(q)` uses the column list set by `Columns`:

```go
sql, args, err := gohan.Insert("archived_orders").
	Columns("id", "total").
	FromSelect(gohan.Select("id", "total").From("orders").Where(gohan.Col("created_at").Lt("2020-01-01"))).
	Build(gohan.Postgres())
// sql: INSERT INTO "archived_orders" ("id", "total") SELECT "id", "total" FROM "orders" WHERE "created_at" < $1
// args: [2020-01-01]
```

## DEFAULT VALUES

`DefaultValues()` inserts a single row that takes every column's default (`FeatureDefaultValues`:
PostgreSQL, SQLite, Generic). It cannot be combined with `Columns`, `Values`, `Rows`, `SetMap` or
`FromSelect` (`ErrInsertMixed`):

```go
sql, args, err := gohan.Insert("d").
	DefaultValues().
	Returning("id", "code", "name").
	Build(gohan.Postgres())
// sql: INSERT INTO "d" DEFAULT VALUES RETURNING "id", "code", "name"
// args: []
```

It composes with `OnConflict` and `Returning` as usual, subject to the same dialect gating as any
other INSERT — with one extra restriction on SQLite and ClickHouse, covered below.

## ON CONFLICT (upsert)

`OnConflict(cols...)` starts an `ON CONFLICT` clause on dialects with `FeatureUpsert` (PostgreSQL
and SQLite). It is followed by one of three actions.

**DoNothing** skips conflicting rows. The conflict target may be empty:

```go
sql, args, err := gohan.Insert("tags").
	Columns("name").
	Values("go").
	OnConflict("name").DoNothing().
	Build(gohan.SQLite())
// sql: INSERT INTO `tags` (`name`) VALUES (?) ON CONFLICT (`name`) DO NOTHING
// args: [go]
```

**DoUpdateExcluded(cols...)** sets each listed column to the value that was about to be inserted.
Every listed column must be one of the INSERT's own columns (`ErrUnknownField`):

```go
sql, args, err := gohan.Insert("users").
	Columns("id", "name", "email").
	Values(1, "alice", "alice@example.com").
	OnConflict("id").
	DoUpdateExcluded("name", "email").
	Build(gohan.Postgres())
// sql: INSERT INTO "users" ("id", "name", "email") VALUES ($1, $2, $3) ON CONFLICT ("id") DO UPDATE SET "name" = excluded."name", "email" = excluded."email"
// args: [1 alice alice@example.com]
```

**DoUpdate(map)** sets explicit values. Keys are sorted. A value that is an expression is rendered
in place: `gohan.Excluded(col)` for the proposed value, `gohan.Raw` for arithmetic on the existing
row. Anything else is bound:

```go
sql, args, err := gohan.Insert("page_views").
	Columns("path", "views", "last_seen").
	Values("/home", 1, "2024-06-01").
	OnConflict("path").
	DoUpdate(map[string]any{
		"views":     gohan.Raw("? + 1", gohan.Col("page_views.views")),
		"last_seen": gohan.Excluded("last_seen"),
	}).
	Build(gohan.Postgres())
// sql: INSERT INTO "page_views" ("path", "views", "last_seen") VALUES ($1, $2, $3) ON CONFLICT ("path") DO UPDATE SET "last_seen" = excluded."last_seen", "views" = "page_views"."views" + 1
// args: [/home 1 2024-06-01]
```

Both update actions need a conflict target; without one, `Build` fails with `ErrConflictTarget`.

### SQLite: INSERT ... SELECT with ON CONFLICT

SQLite's grammar cannot tell a `SELECT`'s trailing clauses from the `ON CONFLICT` clause, so it
requires a `WHERE` on the last select. `gohan` adds `WHERE true` when there is none:

```go
sql, args, err := gohan.Insert("tags").
	Columns("name").
	FromSelect(gohan.Select("name").From("staging_tags")).
	OnConflict("name").DoNothing().
	Build(gohan.SQLite())
// sql: INSERT INTO `tags` (`name`) SELECT `name` FROM `staging_tags` WHERE true ON CONFLICT (`name`) DO NOTHING
// args: []
```

`DefaultValues().OnConflict(...)` is a syntax error on real SQLite, so `gohan` rejects it at
`Build` with `ErrUnsupported` instead; PostgreSQL allows it.

### ClickHouse

ClickHouse has no `ON CONFLICT`: `OnConflict` fails with `ErrUnsupported`. Use a
`ReplacingMergeTree` table and read it with [`Final()`](dialects.md#clickhouse) instead.

```go
_, _, err := gohan.Insert("events").
	Columns("id").
	Values(1).
	OnConflict("id").DoNothing().
	Build(gohan.ClickHouse())
fmt.Println(err)
// Output:
// gohan: not supported by dialect: ON CONFLICT
```

ClickHouse also has no `DEFAULT VALUES` (`INSERT INTO t DEFAULT VALUES` is a syntax error there):
`DefaultValues()` fails with `ErrUnsupported` on `ClickHouse()` and `ClickHouseNamed()`. Use
`Values` with the defaults spelled out explicitly instead.

## RETURNING

`Returning(cols...)` is available on PostgreSQL and SQLite (`FeatureReturning`):

```go
sql, args, err := gohan.Insert("users").
	Columns("name").
	Values("alice").
	Returning("id", "created_at").
	Build(gohan.Postgres())
// sql: INSERT INTO "users" ("name") VALUES ($1) RETURNING "id", "created_at"
// args: [alice]
```

Run it with `QueryContext`/`QueryRowContext`, not `ExecContext`.

## Errors

| Error | Cause |
|---|---|
| `ErrNoColumns` | no row source, `Rows()` with no records, an empty `SetMap`, a record with no insertable fields, `FromSelect` without `Columns`, or an update action with nothing to set |
| `ErrValueCount` | a `Values` row whose length differs from `Columns` (or `Values` without `Columns`) |
| `ErrInsertMixed` | more than one of `Values`, `Rows`, `SetMap`, `FromSelect`, `DefaultValues`; or `DefaultValues` with a non-empty `Columns` |
| `ErrInvalidRecord`, `ErrRecordType`, `ErrRecordShape`, `ErrInconsistentOmit`, `ErrDuplicateColumn` | struct problems in `Rows`; see [Records](records-and-struct-tags.md#errors) |
| `ErrConflictTarget` | `DoUpdate`/`DoUpdateExcluded` without conflict columns |
| `ErrUnknownField` | a `DoUpdateExcluded` column that is not inserted (with `DefaultValues`, always — there are no INSERT columns to match) |
| `ErrUnsupported` | `OnConflict`, `Excluded` or `Returning` on a dialect without the feature; a `Table` with an alias; `DefaultValues` on ClickHouse; `DefaultValues().OnConflict(...)` on SQLite |
| `ErrNoTable` | a missing or empty table, a table of the wrong type, or `FromSelect(nil)` |
| `ErrInvalidIdentifier` | a dotted (qualified) column name: in the column list, a `SetMap` key, a record column, the conflict target, a `DoUpdate` key or `Excluded` |
| `ErrTooManyArgs` | more bound values than the dialect allows |
