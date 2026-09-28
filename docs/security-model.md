# Security model

`gohan` is built so that the normal API cannot produce SQL injection: **values are always bound
and identifiers are always quoted**. This page describes how that holds, where it deliberately
does not, and what stays the caller's responsibility.

To report a vulnerability, see [SECURITY.md](https://github.com/oddbit-project/gohan/blob/main/SECURITY.md).

## What is guaranteed

**Values are never written into the SQL text.** Every Go value in value position (comparison
operands, `IN` lists, `INSERT` values, `SET` values, `Fn`/`Raw` arguments, `CASE` results,
`SETTINGS` values) becomes a placeholder, and the value goes into the argument list for the driver
to bind:

```go
sql, args, err := gohan.From("users").
	Where(gohan.Col("name").Eq("x' OR '1'='1")).
	Build(gohan.Postgres())
// sql: SELECT * FROM "users" WHERE "name" = $1
// args: [x' OR '1'='1]
```

The only values written inline are numbers formatted by `strconv` (`LIMIT`/`OFFSET` and
`SampleRows` as `uint64`, `Int(n)` as `int64`, the validated `Sample` ratio as `float64`) and the
`NULL` keyword from `Val(nil)`, `Eq(nil)` and `Neq(nil)`.

**Identifiers are always quoted and escaped** per dialect, whatever characters they contain:

```go
sql, args, err := gohan.Select(`name"; DROP TABLE users; --`).From("users").Build(gohan.Postgres())
// sql: SELECT "name""; DROP TABLE users; --" FROM "users"
// args: []
```

**Expressions can only be made by this package.** `gohan.Expr` has an unexported method, so no other
package can implement it and inject a fragment of SQL through it. Apart from the escape hatches
below and validated `SETTINGS` names, all SQL text comes from string constants in `gohan` itself.

## Trusted inputs (escape hatches)

Three strings are written into the SQL as they are and must never be built from request data:

- the SQL text of `Raw(sql, ...)`;
- the function name of `Fn(name, ...)`;
- the type of `Cast(v, typ)`.

`gohan` validates their syntax (see [Raw SQL and escape hatches](raw-and-escape-hatches.md)), which
catches mistakes but is not a sanitizer. Their arguments are bound normally.

## Identifiers from request data

Quoting makes any name syntactically safe, but it does not make it *allowed*. If a caller can choose
a column or table name (a sort field, a filter, a "columns" parameter), quoting stops injection but
not access to a column you did not mean to expose (`password_hash`, another tenant's table). Check
such names against an allowlist before passing them to `Col`, `OrderBy`, `Select` or `From`. See
[Identifiers from request data](raw-and-escape-hatches.md#identifiers-from-request-data).

## Pattern matching

`Like(p)` binds `p` as a pattern, so `%` and `_` in it are wildcards. Passing user text to `Like`
cannot inject SQL, but it can widen the match (a search for `%` matches everything). Use
`Contains`, `HasPrefix` or `HasSuffix` (or their case-insensitive `...Fold` forms) for user text;
they escape wildcards per dialect.

## Accidental mass updates

`UPDATE` and `DELETE` without a `WHERE`, or with a `WHERE` that is always true because it was built
from an empty list, fail with `ErrNoWhere` unless `All()` is called. This is a guard against
mistakes, not an access-control mechanism, and it recognizes only specific shapes. See
[The WHERE requirement](update-and-delete.md#the-where-requirement).

## ClickHouse

clickhouse-go binds placeholders on the client by formatting each value into SQL text, so the
safety of a bound value depends on the driver's formatting. `gohan` therefore:

- rejects value types clickhouse-go would not format safely (maps, most structs, `error` and
  `fmt.Formatter` values, including struct-based `driver.Valuer`s nested in slices), with
  `ErrUnsafeValue`;
- rejects identifiers containing `?`, `@`, `{`, `}` or `$` followed by a digit, which could be read
  as parameter syntax;
- rejects `??` in `Raw` text.

These checks were verified against clickhouse-go v2.40.3. See [Dialects](dialects.md#values).

## Resource limits

Every dialect but ClickHouse has a bound-argument limit (65535 for PostgreSQL, 32766 for SQLite,
999 for Generic). A statement over the limit fails with `ErrTooManyArgs` instead of reaching the
database. Bound the size of request-supplied lists (for example `In(ids)`) in your own code.

## Out of scope

- Authorization: which rows and columns a caller may read or change.
- DDL and other SQL you write by hand; `Dialect.QuoteIdent` helps with identifiers there.
- Bugs in database drivers or servers. Report those upstream; a heads-up is welcome if `gohan`
  should defend against one.
