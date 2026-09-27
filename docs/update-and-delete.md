# UPDATE and DELETE

- [UPDATE](#update)
- [Updating from a struct: SetRecord](#updating-from-a-struct-setrecord)
- [DELETE](#delete)
- [The WHERE requirement](#the-where-requirement)
- [RETURNING](#returning)
- [Dialect notes](#dialect-notes)

## UPDATE

`Update(table)` takes a table name or a `gohan.Table(name)` without an alias. `Set(col, v)` adds one
assignment; `SetMap(m)` adds one per key, sorted:

```go
sql, args, err := gohan.Update("users").
	Set("name", "bob").
	Set("active", true).
	Where(gohan.Col("id").Eq(1)).
	Build(gohan.Postgres())
// sql: UPDATE "users" SET "name" = $1, "active" = $2 WHERE "id" = $3
// args: [bob true 1]
```

A value that is an expression is rendered in place, without parentheses, so it can refer to the
current row. Anything else is bound:

```go
sql, args, err := gohan.Update("accounts").
	SetMap(map[string]any{
		"balance":    gohan.Raw("balance - ?", 25),
		"updated_at": gohan.Fn("now"),
		"updated_by": "system",
	}).
	Where(gohan.Col("id").Eq(9)).
	Build(gohan.Postgres())
// sql: UPDATE "accounts" SET "balance" = balance - $1, "updated_at" = now(), "updated_by" = $2 WHERE "id" = $3
// args: [25 system 9]
```

`Set(col, nil)` binds a `nil` argument, which the driver sends as `NULL`. Setting the same column
twice (across `Set`, `SetMap` and `SetRecord`) fails with `ErrDuplicateColumn`, and a dotted
column name fails with `ErrInvalidIdentifier`.

## Updating from a struct: SetRecord

`SetRecord(rec, opts...)` adds one assignment per updatable field of a struct. Fields tagged `auto`
are skipped. Options narrow the field set:

| Option | Effect |
|---|---|
| `gohan.IncludeFields(names...)` | only these fields (Go field name or column name); `ExcludeFields` is then ignored, and `auto` fields stay skipped unless `WithAutoFields()` is also passed |
| `gohan.ExcludeFields(names...)` | every field except these |
| `gohan.SkipZeroValues()` | skip fields holding their type's zero value |
| `gohan.WithAutoFields()` | also write `auto` fields |

```go
type User struct {
	ID    int64  `db:"id" auto:"true"`
	Name  string `db:"name"`
	Email string `db:"email"`
	Role  string `db:"role"`
}
u := User{ID: 7, Name: "alice", Email: "alice@example.com", Role: "admin"}

sql, args, err := gohan.Update("users").
	SetRecord(u, gohan.ExcludeFields("Role")).
	Where(gohan.Col("id").Eq(u.ID)).
	Build(gohan.Postgres())
// sql: UPDATE "users" SET "name" = $1, "email" = $2 WHERE "id" = $3
// args: [alice alice@example.com 7]
```

```go
type User struct {
	ID    int64  `db:"id" auto:"true"`
	Name  string `db:"name"`
	Email string `db:"email"`
}

sql, args, err := gohan.Update("users").
	SetRecord(User{Email: "new@example.com"}, gohan.IncludeFields("email")).
	Where(gohan.Col("id").Eq(7)).
	Build(gohan.Postgres())
// sql: UPDATE "users" SET "email" = $1 WHERE "id" = $2
// args: [new@example.com 7]
```

A name in `IncludeFields`/`ExcludeFields` that matches no field fails with `ErrUnknownField`, so a
typo cannot silently widen the update. `goqu:"omitnil"` and `goqu:"omitempty"` fields are skipped
the same way as on insert. Field values are copied when `SetRecord` is called (a shallow copy:
slice and map fields still share their contents with the record).

## DELETE

`Delete(table)` takes a table name or a `gohan.Table` (an alias is allowed except on ClickHouse):

```go
sql, args, err := gohan.Delete("sessions").
	Where(gohan.Col("expires_at").Lt("2024-01-01")).
	Build(gohan.Postgres())
// sql: DELETE FROM "sessions" WHERE "expires_at" < $1
// args: [2024-01-01]
```

## The WHERE requirement

`Update` and `Delete` refuse to build without a `WHERE` clause, failing with `ErrNoWhere`. This
turns a forgotten filter into an error instead of an update or delete of every row:

```go
_, _, err := gohan.Update("users").Set("active", false).Build(gohan.Postgres())
fmt.Println(errors.Is(err, gohan.ErrNoWhere), err)
// Output:
// true gohan: statement requires a WHERE clause; call All() to affect every row
```

They also refuse a `WHERE` that can only be true, such as one built from `And()` with no arguments,
`NotIn` with an empty list, `Not` of an always-false condition such as `Or()` or an empty `In`,
or `NotExists` of a subquery whose `WHERE` is always false. This catches filters built from an
empty slice:

```go
var ids []int64 // e.g. an empty selection from a request

_, _, err := gohan.Delete("users").Where(gohan.Col("id").NotIn(ids)).Build(gohan.Postgres())
fmt.Println(errors.Is(err, gohan.ErrNoWhere))

_, _, err = gohan.Delete("users").Where(gohan.Not(gohan.Col("id").In(ids))).Build(gohan.Postgres())
fmt.Println(errors.Is(err, gohan.ErrNoWhere))
// Output:
// true
// true
```

The check applies to the combined `WHERE`. A real condition ANDed with an always-true one is
accepted, and affects every row the real condition matches:

```go
sql, args, err := gohan.Delete("users").
	Where(gohan.Col("tenant_id").Eq(3), gohan.Col("status").NotIn()).
	Build(gohan.Postgres())
// sql: DELETE FROM "users" WHERE ("tenant_id" = $1 AND 1=1)
// args: [3]
```

The guard also rejects a condition that references no column at all — a constant such as
`Raw("1=1")`, `Raw("true")` or `Val(1).Eq(1)` — using `IsTrivial` (see
[Checking a condition](expressions.md#checking-a-condition)):

```go
_, _, err := gohan.Delete("users").Where(gohan.Raw("1=1")).Build(gohan.Postgres())
fmt.Println(errors.Is(err, gohan.ErrNoWhere))
// Output:
// true
```

This is stricter than earlier releases: code that relied on `Where(gohan.Raw("1=1"))` (or another
column-free condition) to affect every row now needs `All()` instead.

The column-free rule is syntactic, not semantic: it does not know that `Col("id").Eq(Col("id"))`
is always true, that an uncorrelated `Exists`/subquery is unaffected by the outer row, or that
`Raw("abs(1) = 1")` is a constant (the word `abs` counts as a reference). It is a safety net, not
a substitute for building the condition you mean.

To affect every row on purpose, call `All()`:

```go
sql, args, err := gohan.Update("users").Set("active", false).All().Build(gohan.Postgres())
// sql: UPDATE "users" SET "active" = $1
// args: [false]
```

```go
q := gohan.Delete("scratch").All()

for _, d := range []gohan.Dialect{gohan.Postgres(), gohan.SQLite(), gohan.ClickHouse()} {
	sql, _, _ := q.Build(d)
	fmt.Println(sql)
}
// Output:
// DELETE FROM "scratch"
// DELETE FROM `scratch`
// DELETE FROM "scratch" WHERE 1
```

ClickHouse's `DELETE` requires a `WHERE`, so `All()` renders `WHERE 1` there. `All()` combined with
an always-true `WHERE` renders that `WHERE` as written.

## RETURNING

On PostgreSQL and SQLite, `Returning(cols...)` adds a `RETURNING` clause; run the statement with
`QueryContext`:

```go
sql, args, err := gohan.Update("jobs").
	Set("status", "cancelled").
	Where(gohan.Col("status").Eq("queued")).
	Returning("id").
	Build(gohan.SQLite())
// sql: UPDATE `jobs` SET `status` = ? WHERE `status` = ? RETURNING `id`
// args: [cancelled queued]
```

```go
sql, args, err := gohan.Delete("sessions").
	Where(gohan.Col("user_id").Eq(42)).
	Returning("id", "created_at").
	Build(gohan.Postgres())
// sql: DELETE FROM "sessions" WHERE "user_id" = $1 RETURNING "id", "created_at"
// args: [42]
```

## Dialect notes

- **ClickHouse**: the dialect does not build `UPDATE` (`ErrUnsupported`), including ClickHouse's
  lightweight `UPDATE`, and supports neither `RETURNING` nor a table alias in `DELETE`. `DELETE`
  is ClickHouse's lightweight delete, which does not work on every table engine (for example
  Distributed tables or tables with projections); `gohan` still renders valid SQL, but whether
  the server accepts it depends on the table.
- **Generic** supports `UPDATE` but not `RETURNING`.

```go
_, _, err := gohan.Update("events").Set("x", 1).Where(gohan.Col("id").Eq(1)).Build(gohan.ClickHouse())
fmt.Println(err)

_, _, err = gohan.Delete(gohan.Table("events").As("e")).Where(gohan.Col("e.id").Eq(1)).Build(gohan.ClickHouse())
fmt.Println(err)
// Output:
// gohan: not supported by dialect: UPDATE
// gohan: not supported by dialect: table alias in DELETE
```
