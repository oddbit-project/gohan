# Raw SQL and escape hatches

`gohan` quotes every identifier and binds every value. Three inputs are exceptions, because they are
SQL text by nature:

| Input | Checked | Trust |
|---|---|---|
| `Raw(sql, ...)`: the `sql` text | placeholder syntax only | trusted: written into the SQL verbatim |
| `Fn(name, ...)`: the function name | must match `^[A-Za-z_][A-Za-z0-9_]*$` | trusted: any function can be called |
| `Cast(v, typ)`: the type string | starts with a letter or `_`; then letters, digits, `_`, space, `,`, `()`, `[]`, balanced parentheses | trusted |

The arguments of `Raw` and `Fn`, and the value of `Cast`, are bound like any other value.

**Never build these three strings from request data.** Write them as constants in your code. The
checks stop accidents such as a stray `;`, not a determined attacker.

```go
// Wrong: the column name reaches the SQL unquoted.
gohan.Raw(fmt.Sprintf("lower(%s) = ?", r.URL.Query().Get("field")), v)

// Right: quote identifiers with Col, bind values.
gohan.Fn("lower", gohan.Col(allowedField)).Eq(v)
```

## Raw

`Raw(sql, args...)` writes `sql` as is, replacing each `?` with the next argument. An argument
is bound, unless it is an expression (such as `Col`), in which case it is rendered in place:

```go
sql, args, err := gohan.From("events").
	Where(gohan.Raw("? > now() - interval '1 day' * ?", gohan.Col("created_at"), 7)).
	Build(gohan.Postgres())
// sql: SELECT * FROM "events" WHERE ("created_at" > now() - interval '1 day' * $1)
// args: [7]
```

`??` writes a literal `?` without consuming an argument. PostgreSQL uses `?` as an operator (for
example jsonb's key-exists operator), so this is how to write it:

```go
sql, args, err := gohan.From("docs").Where(gohan.Raw("data ?? ?", "owner")).Build(gohan.Postgres())
// sql: SELECT * FROM "docs" WHERE (data ? $1)
// args: [owner]
```

### Parentheses

A `Raw` expression is wrapped in parentheses when it is an operand (of `And`, `Or`, `Not`, a
comparison), a select-list column or an `ORDER BY` item, so it cannot change the meaning of the
SQL around it. The exception is a `Raw` passed as an argument of another `Raw`: it is inserted as
is, so write any parentheses it needs in the outer text:

```go
sql, args, err := gohan.Select(gohan.Raw("price * ?", 2).As("double")).
	From("items").
	Where(gohan.Raw("a OR b"), gohan.Col("c").Eq(1)).
	OrderBy(gohan.Raw("random()")).
	Build(gohan.Postgres())
// sql: SELECT (price * $1) AS "double" FROM "items" WHERE ((a OR b) AND "c" = $2) ORDER BY (random()) ASC
// args: [2 1]
```

As the value of an `UPDATE ... SET` or `ON CONFLICT DO UPDATE SET` assignment it is written without
parentheses:

```go
sql, args, err := gohan.Update("counters").
	Set("n", gohan.Raw("n + ?", 1)).
	Where(gohan.Col("id").Eq(1)).
	Build(gohan.SQLite())
// sql: UPDATE `counters` SET `n` = n + ? WHERE `id` = ?
// args: [1 1]
```

### Rejected placeholder sequences

`Raw` text must not contain anything a database driver could read as its own placeholder, since
that would bind arguments out of order. `Build` fails with `ErrRawPlaceholder` for:

- `$` followed by a digit (PostgreSQL's `$1` style), anywhere in the text;
- `?` or `??` followed by a digit (SQLite reads `?1` as a numbered placeholder);
- `?` directly after `-`, `$` or `\`;
- `??` on ClickHouse, which has no way to write a literal `?` in `Raw` text.

A number of `?` markers that differs from the number of arguments fails with `ErrRawArgs`.

```go
_, _, err := gohan.From("t").Where(gohan.Raw("a = $1", 5)).Build(gohan.Postgres())
fmt.Println(errors.Is(err, gohan.ErrRawPlaceholder))

_, _, err = gohan.From("t").Where(gohan.Raw("a = ? AND b = ?", 1)).Build(gohan.Postgres())
fmt.Println(errors.Is(err, gohan.ErrRawArgs))

_, _, err = gohan.From("t").Where(gohan.Raw("x ?? y")).Build(gohan.ClickHouse())
fmt.Println(errors.Is(err, gohan.ErrRawPlaceholder))
// Output:
// true
// true
// true
```

## Fn

`Fn(name, args...)` renders `name(args...)`. Arguments are bound unless they are expressions:

```go
sql, args, err := gohan.From("users").
	Where(gohan.Fn("lower", gohan.Col("email")).Eq("alice@example.com")).
	Build(gohan.Postgres())
// sql: SELECT * FROM "users" WHERE lower("email") = $1
// args: [alice@example.com]
```

A name outside `^[A-Za-z_][A-Za-z0-9_]*$` fails with `ErrInvalidFunction`. Schema-qualified names
such as `pg_catalog.lower` are therefore not accepted; use `Raw` for those.

```go
_, _, err := gohan.Select(gohan.Fn("lower(email); --")).From("users").Build(gohan.Postgres())
fmt.Println(err)
// Output:
// gohan: invalid function name: "lower(email); --"
```

## Cast

`Cast(v, typ)` renders `CAST(v AS typ)`. `v` is in value position: a string is bound, so pass `Col`
for a column. `typ` is checked (`ErrInvalidType`) and then written as is:

```go
sql, args, err := gohan.Select(gohan.Cast("2024-01-01", "date").As("d")).Build(gohan.Postgres())
// sql: SELECT CAST($1 AS date) AS "d"
// args: [2024-01-01]
```

```go
_, _, err := gohan.Select(gohan.Cast(gohan.Col("x"), "int); DROP TABLE t; --")).From("t").Build(gohan.Postgres())
fmt.Println(err)
// Output:
// gohan: invalid cast type: "int); DROP TABLE t; --"
```

On PostgreSQL, `Cast` is also the way to give a bound value in a select list a type: a bare `$1`
there is sent as `text`, so a Go `int` cannot be encoded for it.

## Identifiers for hand-written SQL

When you write DDL or other SQL by hand, `Dialect.QuoteIdent` quotes a name with the same rules
`gohan` uses internally:

```go
q, err := gohan.Postgres().QuoteIdent("audit.log")
fmt.Println(q, err)
// Output:
// "audit"."log" <nil>
```

## Identifiers from request data

Quoting makes any string a syntactically safe identifier, but it does not decide which columns a
caller may see or sort by. When a column or table name comes from a request (a sort field, a
filter name), check it against an allowlist first:

```go
sortable := map[string]bool{"name": true, "created_at": true}

field := "created_at" // from the request
if !sortable[field] {
	field = "name"
}
sql, args, err := gohan.From("users").OrderBy(gohan.Col(field).Desc()).Build(gohan.Postgres())
// sql: SELECT * FROM "users" ORDER BY "created_at" DESC
// args: []
```

The `field` package's `grid:"sort"`, `grid:"filter"` and `grid:"search"` tags can drive such an
allowlist; see [Records and struct tags](records-and-struct-tags.md).
