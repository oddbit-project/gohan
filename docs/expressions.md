# Expressions

- [Columns and values](#columns-and-values)
- [Comparisons](#comparisons)
- [NULL](#null)
- [IN and NOT IN](#in-and-not-in)
- [LIKE and pattern helpers](#like-and-pattern-helpers)
- [Combining conditions](#combining-conditions)
- [Checking a condition](#checking-a-condition)
- [Aggregates and functions](#aggregates-and-functions)
- [CASE](#case)
- [Integer literals: Int](#integer-literals-int)
- [Subquery expressions](#subquery-expressions)
- [Aliases and ordering](#aliases-and-ordering)

Expressions are values of type `gohan.Value` (or `gohan.Order` for `ORDER BY` items). They are only
created by this package, so every fragment of SQL comes from a constructor that quotes or binds its
input. The exceptions are `Raw`'s text, `Fn`'s name and `Cast`'s type; see
[Raw SQL and escape hatches](raw-and-escape-hatches.md).

## Columns and values

`Col(name)` is a quoted column reference (`"table.column"` is split on dots). `Val(v)` is a bound
value. Where an expression is expected, a plain Go value in value position is bound as if wrapped
in `Val` (see the [string-position rule](getting-started.md#the-string-position-rule)).

`Val` is needed when a value must appear where a column name would otherwise be assumed, for
example on the left of a comparison:

```go
sql, args, err := gohan.From("users").
	Where(gohan.Val("admin").In(gohan.Col("role"), gohan.Col("backup_role"))).
	Build(gohan.Postgres())
// sql: SELECT * FROM "users" WHERE $1 IN ("role", "backup_role")
// args: [admin]
```

`Val(nil)` renders the `NULL` keyword and `Val(expr)` returns `expr` unchanged. A plain `nil`
passed elsewhere in value position (an `Fn` argument, a `Set` value, a `CASE` result) is bound as
a nil argument instead, except in `Eq`/`Neq`, which render `IS NULL`/`IS NOT NULL`. `Star()` renders an
unquoted `*`.

## Comparisons

Methods on `Value`: `Eq`, `Neq`, `Gt`, `Gte`, `Lt`, `Lte`, `Between`, `NotBetween`. The argument is
bound unless it is an expression:

```go
sql, args, err := gohan.From("orders").
	Where(
		gohan.Col("total").Between(10, 100),
		gohan.Col("shipped_at").Gt(gohan.Col("paid_at")),
		gohan.Col("status").Neq("cancelled"),
	).
	Build(gohan.SQLite())
// sql: SELECT * FROM `orders` WHERE (`total` BETWEEN ? AND ? AND `shipped_at` > `paid_at` AND `status` <> ?)
// args: [10 100 cancelled]
```

## NULL

`Eq(nil)` and `Neq(nil)` render `IS NULL` and `IS NOT NULL`, because `= NULL` never matches. A nil
pointer counts as nil. `IsNull()` and `IsNotNull()` say the same thing explicitly:

```go
var deletedAt *time.Time

sql, args, err := gohan.From("users").
	Where(gohan.Col("deleted_at").Eq(deletedAt), gohan.Col("email").IsNotNull()).
	Build(gohan.Postgres())
// sql: SELECT * FROM "users" WHERE ("deleted_at" IS NULL AND "email" IS NOT NULL)
// args: []
```

## IN and NOT IN

`In(values...)` and `NotIn(values...)` bind one placeholder per value. A single slice argument
(other than `[]byte` or a `driver.Valuer`) is expanded:

```go
ids := []int64{3, 5, 8}

sql, args, err := gohan.From("users").Where(gohan.Col("id").In(ids)).Build(gohan.Postgres())
// sql: SELECT * FROM "users" WHERE "id" IN ($1, $2, $3)
// args: [3 5 8]
```

An empty list renders a constant: `In()` is always false (`1=0`) and `NotIn()` is always true
(`1=1`). An `UPDATE` or `DELETE` whose whole `WHERE` is always true is refused; see
[the WHERE requirement](update-and-delete.md#the-where-requirement).

```go
sql, args, err := gohan.From("users").Where(gohan.Col("id").In([]int64{})).Build(gohan.Postgres())
// sql: SELECT * FROM "users" WHERE 1=0
// args: []
```

A single `*SelectBuilder` argument renders a subquery; see [Subqueries](select.md#subqueries).

## LIKE and pattern helpers

`Like(p)` and `NotLike(p)` bind the pattern as given: `%` and `_` in it are wildcards. Use them for
patterns you write yourself.

For text that comes from a user, use `Contains(s)`, `HasPrefix(s)` or `HasSuffix(s)`. They escape
`%`, `_` and the escape character in `s`, so the text is matched literally. The escaping differs
per dialect: ClickHouse uses backslashes, the others use `!` with an explicit `ESCAPE '!'`:

```go
q := gohan.From("products").Where(gohan.Col("name").Contains("50%_off"))

for _, d := range []gohan.Dialect{gohan.Postgres(), gohan.SQLite(), gohan.ClickHouse()} {
	sql, args, _ := q.Build(d)
	fmt.Println(sql, args)
}
// Output:
// SELECT * FROM "products" WHERE "name" LIKE $1 ESCAPE '!' [%50!%!_off%]
// SELECT * FROM `products` WHERE `name` LIKE ? ESCAPE '!' [%50!%!_off%]
// SELECT * FROM "products" WHERE "name" LIKE ? [%50\%\_off%]
```

`ILike(p)` and `NotILike(p)` render `ILIKE` on PostgreSQL and ClickHouse and fail with
`ErrUnsupported` elsewhere. Case sensitivity of plain `LIKE` differs: SQLite's `LIKE` is
case-insensitive for ASCII letters, while PostgreSQL's and ClickHouse's are case-sensitive.

```go
sql, args, err := gohan.From("users").Where(gohan.Col("name").ILike("ann%")).Build(gohan.Postgres())
// sql: SELECT * FROM "users" WHERE "name" ILIKE $1
// args: [ann%]
```

`ContainsFold(s)`, `HasPrefixFold(s)` and `HasSuffixFold(s)` are the case-insensitive forms of
`Contains`, `HasPrefix` and `HasSuffix`, with the same escaping. They render `ILIKE` on PostgreSQL
and ClickHouse, `LIKE` on SQLite, and fail with `ErrUnsupported` on Generic. SQLite's `LIKE` folds
ASCII letters only, so `ContainsFold("école")` matches `ÉCOLE` on PostgreSQL and ClickHouse but not
on SQLite:

```go
q := gohan.From("products").Where(gohan.Col("name").ContainsFold("50%_Off"))

for _, d := range []gohan.Dialect{gohan.Postgres(), gohan.SQLite(), gohan.ClickHouse()} {
	sql, args, _ := q.Build(d)
	fmt.Println(sql, args)
}
// Output:
// SELECT * FROM "products" WHERE "name" ILIKE $1 ESCAPE '!' [%50!%!_Off%]
// SELECT * FROM `products` WHERE `name` LIKE ? ESCAPE '!' [%50!%!_Off%]
// SELECT * FROM "products" WHERE "name" ILIKE ? [%50\%\_Off%]
```

## Combining conditions

`And(exprs...)` and `Or(exprs...)` join their arguments in parentheses (a single argument is
rendered as is). `Not(expr)` negates:

```go
sql, args, err := gohan.From("t").
	Where(gohan.Or(
		gohan.And(gohan.Col("a").Eq(1), gohan.Col("b").Eq(2)),
		gohan.Not(gohan.Col("c").Eq(3)),
	)).
	Build(gohan.Postgres())
// sql: SELECT * FROM "t" WHERE (("a" = $1 AND "b" = $2) OR NOT ("c" = $3))
// args: [1 2 3]
```

With no arguments, `And()` renders the always-true `(1=1)` and `Or()` the always-false `(1=0)`.

`Match(map)` is an `AND` of equality tests, with keys sorted. An empty map fails with
`ErrEmptyMatch`:

```go
sql, args, err := gohan.From("users").
	Where(gohan.Match(map[string]any{"status": "active", "country": "PT"})).
	Build(gohan.Postgres())
// sql: SELECT * FROM "users" WHERE ("country" = $1 AND "status" = $2)
// args: [PT active]
```

## Checking a condition

`IsEmpty(cond)` reports whether `cond` places no condition at all: `nil`, the zero `Value`, or
`And()` with no elements (directly, or nested only in other empty `And`s).

`IsTrivial(cond)` reports whether `cond` does not restrict rows: it is empty, syntactically
always true (as `And()`, `Or(x, And())`, `Not(Or())` and similar already are), or it references
no column at all — a constant such as `Raw("1=1")`, `Raw("true")`, `Raw("? = ?", 1, 1)` or
`Val(1).Eq(1)`:

```go
fmt.Println(gohan.IsTrivial(nil))
fmt.Println(gohan.IsTrivial(gohan.And()))
fmt.Println(gohan.IsTrivial(gohan.Raw("1=1")))
fmt.Println(gohan.IsTrivial(gohan.Col("a").Eq(1)))
// Output:
// true
// true
// true
// false
```

Both functions are pure — they never mutate `cond` and take no dialect, since column-ness does not
depend on the dialect. `Update`/`Delete`'s `WHERE` guard (see
[The WHERE requirement](update-and-delete.md#the-where-requirement)) uses `IsTrivial` directly.

The column-free check is syntactic: it does not know that `Col("id").Eq(Col("id"))` is always
true, that an uncorrelated subquery in `Exists`/`In` is unaffected by the outer row (it still
references a table, so it is not column-free), or that `Raw("abs(1) = 1")` is a constant (the
word `abs` counts as a reference). These are known limits, not bugs to work around.

## Aggregates and functions

`CountAll()` renders `COUNT(*)`. `Count`, `Sum`, `Avg`, `Min` and `Max` take a column name (a
string) or an expression; any other value is bound (`Count(1)` renders `COUNT($1)` on
PostgreSQL).

`Fn(name, args...)` calls any SQL function. The arguments are in value position, so pass `Col` for
columns. The name is trusted input, checked against `^[A-Za-z_][A-Za-z0-9_]*$`
(`ErrInvalidFunction`):

```go
day := gohan.Fn("date_trunc", "day", gohan.Col("created_at"))

sql, args, err := gohan.Select(day.As("day"), gohan.CountAll().As("n")).
	From("events").
	GroupBy("day").
	Build(gohan.Postgres())
// sql: SELECT date_trunc($1, "created_at") AS "day", COUNT(*) AS "n" FROM "events" GROUP BY "day"
// args: [day]
```

Each use of an expression binds its own arguments. Repeating `day` in `GroupBy` would render
`date_trunc($1, ...)` in the select list and `date_trunc($2, ...)` in `GROUP BY`, which PostgreSQL
does not treat as the same expression, so the query fails. Group by the output alias, as above,
instead.

`Cast(v, type)` renders `CAST(v AS type)`. `v` is in value position; the type string is trusted
input (see [Raw SQL and escape hatches](raw-and-escape-hatches.md)):

```go
sql, args, err := gohan.Select(gohan.Cast(gohan.Col("price"), "numeric(10,2)")).From("items").Build(gohan.Postgres())
// sql: SELECT CAST("price" AS numeric(10,2)) FROM "items"
// args: []
```

## CASE

`Case()` starts a `CASE` expression. `When(cond, then)` adds a branch, and `Else(v)` or `End()`
finishes it. `THEN` and `ELSE` values are in value position, so strings are bound:

```go
sql, args, err := gohan.Select(
	"id",
	gohan.Case().
		When(gohan.Col("score").Gte(90), "A").
		When(gohan.Col("score").Gte(80), "B").
		Else("C").As("grade"),
).From("exams").Build(gohan.Postgres())
// sql: SELECT "id", CASE WHEN "score" >= $1 THEN $2 WHEN "score" >= $3 THEN $4 ELSE $5 END AS "grade" FROM "exams"
// args: [90 A 80 B C]
```

A `CASE` without `When` fails with `ErrEmptyCase`.

## Integer literals: Int

`Int(n)` writes an integer inline instead of binding it (negative numbers are parenthesized). Use it
where the database must know the type from the SQL text, for example constants inside an
aggregate on PostgreSQL, which types a bare `$n` in a `CASE` as `text` and then rejects
`SUM(text)`:

```go
sql, args, err := gohan.Select(
	gohan.Sum(gohan.Case().
		When(gohan.Col("status").Eq("done"), gohan.Int(1)).
		Else(gohan.Int(0))).As("done_count"),
).From("tasks").Build(gohan.Postgres())
// sql: SELECT SUM(CASE WHEN "status" = $1 THEN 1 ELSE 0 END) AS "done_count" FROM "tasks"
// args: [done]
```

## Subquery expressions

- `Sub(q)`: a scalar subquery, `(SELECT ...)`
- `Exists(q)` / `NotExists(q)`: `EXISTS (SELECT ...)`
- `Col(x).In(q)` / `Col(x).NotIn(q)`: `x IN (SELECT ...)`

The subquery shares the outer statement's placeholder numbering. Examples are in
[SELECT: Subqueries](select.md#subqueries).

## Aliases and ordering

`Value.As(alias)` renders `expr AS "alias"`; the alias must be a single, undotted name.
`Value.Asc()` and `Value.Desc()` return an `Order` for `OrderBy`, and `Order.NullsFirst()` /
`Order.NullsLast()` extend it:

```go
sql, args, err := gohan.Select(gohan.Fn("upper", gohan.Col("name")).As("name_uc")).
	From("users").
	OrderBy(gohan.Col("name").Asc().NullsFirst()).
	Build(gohan.SQLite())
// sql: SELECT upper(`name`) AS `name_uc` FROM `users` ORDER BY `name` ASC NULLS FIRST
// args: []
```
