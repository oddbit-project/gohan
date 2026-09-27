# Getting started

## Install

```
go get github.com/oddbit-project/gohan
```

`gohan` needs Go 1.26.5 or later and has no runtime dependencies. It never opens a connection: bring
your own driver (`pgx`, `modernc.org/sqlite`, `clickhouse-go`, ...).

## A first query

Every statement starts with a constructor (`Select`, `From`, `Insert`, `Update`, `Delete`), is
refined with method calls, and is rendered with `Build(dialect)`:

```go
sql, args, err := gohan.Select("id", "name").
	From("users").
	Where(gohan.Col("active").Eq(true)).
	OrderBy("name").
	Limit(10).
	Build(gohan.Postgres())
// sql: SELECT "id", "name" FROM "users" WHERE "active" = $1 ORDER BY "name" ASC LIMIT 10
// args: [true]
```

`Build` returns the SQL text, the arguments to bind (in placeholder order) and an error. The
identifiers are quoted, the value `true` is a placeholder, and `LIMIT` is an integer written inline.

The same builder renders differently per dialect, which is the only thing that changes:

```go
q := gohan.Select("id", "name").From("users").Where(gohan.Col("active").Eq(true))

for _, d := range []gohan.Dialect{gohan.Postgres(), gohan.SQLite(), gohan.ClickHouse()} {
	sql, args, _ := q.Build(d)
	fmt.Println(sql, args)
}
// Output:
// SELECT "id", "name" FROM "users" WHERE "active" = $1 [true]
// SELECT `id`, `name` FROM `users` WHERE `active` = ? [true]
// SELECT "id", "name" FROM "users" WHERE "active" = ? [true]
```

## Running it with database/sql

`gohan` output goes straight into `database/sql`:

```go
query, args, err := gohan.Select("id", "name").
	From("users").
	Where(gohan.Col("id").Eq(42)).
	Build(gohan.Postgres())
if err != nil {
	return err
}
var u User
err = db.QueryRowContext(ctx, query, args...).Scan(&u.ID, &u.Name)
```

See [examples/](https://github.com/oddbit-project/gohan/tree/main/examples) for complete programs against SQLite, PostgreSQL and ClickHouse.

## Choosing a dialect

Use a constructor (`gohan.Postgres()`, `gohan.SQLite()`, `gohan.ClickHouse()`, `gohan.Generic()`)
or look one up by the `database/sql` driver name you passed to `sql.Open`:

```go
d, err := gohan.DialectFor("pgx")
fmt.Println(d.Name(), err)

_, err = gohan.DialectFor("mysql")
fmt.Println(err)
// Output:
// postgres <nil>
// gohan: unknown or zero dialect: "mysql"
```

The pre-registered names are `pgx`, `pgx/v5`, `postgres`, `sqlite`, `sqlite3` and `clickhouse`.
`gohan.Register(name, dialect)` adds or replaces one. See [Dialects](dialects.md).

## Builders are immutable

Every method returns a new builder and leaves its receiver unchanged, so a base query can be
shared and extended safely:

```go
base := gohan.From("orders").Where(gohan.Col("tenant_id").Eq(7))

paid := base.Where(gohan.Col("status").Eq("paid"))

sql1, args1, _ := base.Build(gohan.Postgres())
sql2, args2, _ := paid.Build(gohan.Postgres())
fmt.Println(sql1, args1)
fmt.Println(sql2, args2)
// Output:
// SELECT * FROM "orders" WHERE "tenant_id" = $1 [7]
// SELECT * FROM "orders" WHERE ("tenant_id" = $1 AND "status" = $2) [7 paid]
```

## Errors surface at Build

Builder methods do not return errors. Problems (an invalid identifier, a missing table, a value
count mismatch, ...) are recorded and reported by `Build`, which then returns an empty string, no
arguments and an error wrapping one of the `Err*` values. Check it with `errors.Is`:

```go
sql, args, err := gohan.Delete("users").Build(gohan.Postgres())
fmt.Printf("%q %v\n", sql, args)
fmt.Println(errors.Is(err, gohan.ErrNoWhere), err)
// Output:
// "" []
// true gohan: statement requires a WHERE clause; call All() to affect every row
```

The first error wins. See [Errors](errors.md) for the full list.

## The string-position rule

Whether a Go `string` is a column name or a value depends on where it appears:

- **Column position**: select-list columns, `GROUP BY`, `ORDER BY`, `INSERT`/`UPDATE` column names,
  and the argument of `Count`, `Sum`, `Avg`, `Min`, `Max`. A string here is a column name.
- **Value position**: comparison right-hand sides, `Fn` and `Raw` arguments, `Cast`'s value,
  `Case`'s `THEN`/`ELSE`, and `Val`. A string here is a bound value.

Use `gohan.Col("x")` for a column in value position and `gohan.Val(x)` for a value in column
position:

```go
sql, args, err := gohan.From("t").Where(gohan.Col("a").Eq("b")).Build(gohan.Postgres())
// sql: SELECT * FROM "t" WHERE "a" = $1
// args: [b]
```

```go
sql, args, err := gohan.From("t").Where(gohan.Col("a").Eq(gohan.Col("b"))).Build(gohan.Postgres())
// sql: SELECT * FROM "t" WHERE "a" = "b"
// args: []
```

## Next steps

- [SELECT](select.md), [INSERT and upsert](insert-and-upsert.md),
  [UPDATE and DELETE](update-and-delete.md)
- [Expressions](expressions.md) for everything that goes in a `WHERE`
- [Security model](security-model.md) before exposing any query to request data
