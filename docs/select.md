# SELECT

- [Select list](#select-list)
- [FROM](#from)
- [WHERE](#where)
- [ORDER BY, LIMIT and OFFSET](#order-by-limit-and-offset)
- [DISTINCT](#distinct)
- [Joins](#joins)
- [GROUP BY and HAVING](#group-by-and-having)
- [Subqueries](#subqueries)
- [Common table expressions](#common-table-expressions)
- [UNION](#union)

ClickHouse-only clauses (`FINAL`, `SAMPLE`, `ARRAY JOIN`, `PREWHERE`, `SETTINGS`) are covered in
[Dialects](dialects.md#clickhouse).

## Select list

`Select(cols...)` takes column names (strings) and expressions. `From(table)` is shorthand for
`Select().From(table)`, which selects `*`:

```go
sql, args, err := gohan.From("public.users").Build(gohan.Postgres())
// sql: SELECT * FROM "public"."users"
// args: []
```

Strings are column names and may be qualified with dots; each part is quoted separately. Anything
else must be an expression; `Value.As` adds an alias:

```go
sql, args, err := gohan.Select(
	"u.id",
	gohan.Fn("lower", gohan.Col("u.email")).As("email"),
	gohan.CountAll().As("n"),
).From(gohan.Table("users").As("u")).GroupBy("u.id", "u.email").Build(gohan.Postgres())
// sql: SELECT "u"."id", lower("u"."email") AS "email", COUNT(*) AS "n" FROM "users" AS "u" GROUP BY "u"."id", "u"."email"
// args: []
```

`Columns(cols...)` replaces the select list and `AddColumns(cols...)` appends to it. A value that
is neither a string nor an expression (for example an `int`) fails with `ErrInvalidColumn`; use
`gohan.Int(n)` or `gohan.Val(v)` for constants.

## FROM

The source is a table name (`"schema.table"` allowed), a `gohan.Table(name).As(alias)`, or an
aliased subquery (see [Subqueries](#subqueries)). `TableRef.Col` qualifies a column by the alias:

```go
u := gohan.Table("app.users").As("u")
sql, args, err := gohan.Select(u.Col("id"), u.Col("name")).From(u).Where(u.Col("id").Eq(1)).Build(gohan.Postgres())
// sql: SELECT "u"."id", "u"."name" FROM "app"."users" AS "u" WHERE "u"."id" = $1
// args: [1]
```

## WHERE

`Where(conds...)` takes one or more boolean expressions. Repeated calls and multiple arguments are
ANDed together:

```go
sql, args, err := gohan.Select("id", "total").
	From("orders").
	Where(gohan.Col("status").Eq("paid")).
	Where(gohan.Or(
		gohan.Col("total").Gte(100),
		gohan.Col("customer_tier").In("gold", "platinum"),
	)).
	Build(gohan.Postgres())
// sql: SELECT "id", "total" FROM "orders" WHERE ("status" = $1 AND ("total" >= $2 OR "customer_tier" IN ($3, $4)))
// args: [paid 100 gold platinum]
```

Placeholders are numbered in the order they appear in the final SQL, across subqueries and CTEs.
All comparison and boolean helpers are listed in [Expressions](expressions.md).

## ORDER BY, LIMIT and OFFSET

`OrderBy` takes column names (ascending), `Order` values from `Asc()`/`Desc()`, or `Value`
expressions (ascending); anything else fails with `ErrInvalidColumn`. `NullsFirst()`/`NullsLast()` extend an `Order`:

```go
sql, args, err := gohan.From("users").
	OrderBy("last_name", gohan.Col("last_login").Desc().NullsLast()).
	Limit(20).
	Offset(40).
	Build(gohan.Postgres())
// sql: SELECT * FROM "users" ORDER BY "last_name" ASC, "last_login" DESC NULLS LAST LIMIT 20 OFFSET 40
// args: []
```

`LIMIT` and `OFFSET` are written inline as integers (not bound). `Limit(0)` renders `LIMIT 0`. On
SQLite, whose grammar has no `OFFSET` without `LIMIT`, an offset alone gets `LIMIT -1`:

```go
sql, args, err := gohan.From("users").Offset(40).Build(gohan.SQLite())
// sql: SELECT * FROM `users` LIMIT -1 OFFSET 40
// args: []
```

Values above `math.MaxInt64` fail with `ErrInvalidLimit` on every dialect except ClickHouse.

## DISTINCT

```go
sql, args, err := gohan.Select("country").From("customers").Distinct().Build(gohan.Postgres())
// sql: SELECT DISTINCT "country" FROM "customers"
// args: []
```

## Joins

| Method | SQL |
|---|---|
| `Join(t, on)` | `INNER JOIN t ON on` |
| `LeftJoin(t, on)` | `LEFT JOIN t ON on` |
| `RightJoin(t, on)` | `RIGHT JOIN t ON on` |
| `FullJoin(t, on)` | `FULL JOIN t ON on` |
| `CrossJoin(t)` | `CROSS JOIN t` |
| `JoinUsing(t, cols...)` | `INNER JOIN t USING (cols)` |
| `LeftJoinUsing(t, cols...)` | `LEFT JOIN t USING (cols)` |

`t` is anything `From` accepts. Compare two columns with `Col(...)` on both sides; a bare string on
the right is a bound value:

```go
u := gohan.Table("users").As("u")
o := gohan.Table("orders").As("o")

sql, args, err := gohan.Select(u.Col("name"), o.Col("total")).
	From(u).
	Join(o, o.Col("user_id").Eq(u.Col("id"))).
	Where(o.Col("total").Gt(100)).
	Build(gohan.Postgres())
// sql: SELECT "u"."name", "o"."total" FROM "users" AS "u" INNER JOIN "orders" AS "o" ON "o"."user_id" = "u"."id" WHERE "o"."total" > $1
// args: [100]
```

```go
sql, args, err := gohan.Select("users.id", gohan.Count("orders.id").As("orders")).
	From("users").
	LeftJoin("orders", gohan.Col("orders.user_id").Eq(gohan.Col("users.id"))).
	GroupBy("users.id").
	Build(gohan.SQLite())
// sql: SELECT `users`.`id`, COUNT(`orders`.`id`) AS `orders` FROM `users` LEFT JOIN `orders` ON `orders`.`user_id` = `users`.`id` GROUP BY `users`.`id`
// args: []
```

```go
sql, args, err := gohan.From("orders").JoinUsing("customers", "customer_id").Build(gohan.Postgres())
// sql: SELECT * FROM "orders" INNER JOIN "customers" USING ("customer_id")
// args: []
```

`gohan` does not check whether the target database supports a join type (for example `FULL JOIN`
on older SQLite versions). On ClickHouse, see the [`LEFT JOIN` note](dialects.md#clickhouse) about
`join_use_nulls`.

## GROUP BY and HAVING

`GroupBy` takes column names or expressions; `Having` works like `Where`. The aggregate helpers
`Count`, `Sum`, `Avg`, `Min` and `Max` treat a string argument as a column name, and `CountAll()`
renders `COUNT(*)`:

```go
sql, args, err := gohan.Select(
	"customer_id",
	gohan.CountAll().As("orders"),
	gohan.Sum("total").As("revenue"),
).
	From("orders").
	GroupBy("customer_id").
	Having(gohan.Sum("total").Gt(1000)).
	OrderBy(gohan.Sum("total").Desc()).
	Build(gohan.Postgres())
// sql: SELECT "customer_id", COUNT(*) AS "orders", SUM("total") AS "revenue" FROM "orders" GROUP BY "customer_id" HAVING SUM("total") > $1 ORDER BY SUM("total") DESC
// args: [1000]
```

## Subqueries

A `*SelectBuilder` can be used in four places.

**As a FROM or JOIN source**, through `As(alias)`. The alias is required; passing the builder
itself fails with `ErrNeedAlias`:

```go
recent := gohan.Select("user_id", gohan.Max("created_at").As("last_order")).
	From("orders").
	Where(gohan.Col("status").Eq("paid")).
	GroupBy("user_id").
	As("r")

sql, args, err := gohan.From(recent).
	Where(gohan.Col("r.last_order").Lt("2024-01-01")).
	Build(gohan.Postgres())
// sql: SELECT * FROM (SELECT "user_id", MAX("created_at") AS "last_order" FROM "orders" WHERE "status" = $1 GROUP BY "user_id") AS "r" WHERE "r"."last_order" < $2
// args: [paid 2024-01-01]
```

**As a scalar value**, through `gohan.Sub(q)`:

```go
avg := gohan.Select(gohan.Avg("total")).From("orders").Where(gohan.Col("status").Eq("paid"))

sql, args, err := gohan.Select("id", "total").
	From("orders").
	Where(gohan.Col("total").Gt(gohan.Sub(avg))).
	Build(gohan.Postgres())
// sql: SELECT "id", "total" FROM "orders" WHERE "total" > (SELECT AVG("total") FROM "orders" WHERE "status" = $1)
// args: [paid]
```

**On the right of `In`/`NotIn`**, as the only argument:

```go
banned := gohan.Select("user_id").From("bans").Where(gohan.Col("reason").Eq("spam"))

sql, args, err := gohan.From("comments").
	Where(gohan.Col("user_id").In(banned), gohan.Col("hidden").Eq(false)).
	Build(gohan.Postgres())
// sql: SELECT * FROM "comments" WHERE ("user_id" IN (SELECT "user_id" FROM "bans" WHERE "reason" = $1) AND "hidden" = $2)
// args: [spam false]
```

**In `Exists`/`NotExists`**:

```go
sql, args, err := gohan.From(gohan.Table("users").As("u")).
	Where(gohan.NotExists(
		gohan.Select(gohan.Int(1)).From("orders").
			Where(gohan.Col("orders.user_id").Eq(gohan.Col("u.id"))),
	)).
	Build(gohan.SQLite())
// sql: SELECT * FROM `users` AS `u` WHERE NOT EXISTS (SELECT 1 FROM `orders` WHERE `orders`.`user_id` = `u`.`id`)
// args: []
```

A builder passed anywhere else a value is expected (for example `Eq(q)`) fails with
`ErrInvalidColumn`: wrap it in `Sub`.

## Common table expressions

`With(name, q)` adds a CTE; `WithRecursive(name, q)` makes the whole `WITH` clause `RECURSIVE`:

```go
active := gohan.Select("id", "email").From("users").Where(gohan.Col("active").Eq(true))

sql, args, err := gohan.Select("email").
	With("active_users", active).
	From("active_users").
	Where(gohan.Col("email").HasSuffix("@example.com")).
	Build(gohan.Postgres())
// sql: WITH "active_users" AS (SELECT "id", "email" FROM "users" WHERE "active" = $1) SELECT "email" FROM "active_users" WHERE "email" LIKE $2 ESCAPE '!'
// args: [true %@example.com]
```

```go
tree := gohan.Select("id", "parent_id").From("categories").
	Where(gohan.Col("id").Eq(1)).
	UnionAll(
		gohan.Select("c.id", "c.parent_id").
			From(gohan.Table("categories").As("c")).
			Join("tree", gohan.Col("c.parent_id").Eq(gohan.Col("tree.id"))),
	)

sql, args, err := gohan.Select("id").
	WithRecursive("tree", tree).
	From("tree").
	Build(gohan.Postgres())
// sql: WITH RECURSIVE "tree" AS (SELECT "id", "parent_id" FROM "categories" WHERE "id" = $1 UNION ALL SELECT "c"."id", "c"."parent_id" FROM "categories" AS "c" INNER JOIN "tree" ON "c"."parent_id" = "tree"."id") SELECT "id" FROM "tree"
// args: [1]
```

## UNION

`Union(q)` and `UnionAll(q)` append a member. `ORDER BY`, `LIMIT` and `OFFSET` set on the outer
builder apply to the whole compound:

```go
q := gohan.Select("id", "created_at").From("invoices").
	UnionAll(gohan.Select("id", "created_at").From("credit_notes")).
	OrderBy(gohan.Col("created_at").Desc()).
	Limit(10)

for _, d := range []gohan.Dialect{gohan.Postgres(), gohan.ClickHouse()} {
	sql, _, _ := q.Build(d)
	fmt.Println(sql)
}
// Output:
// SELECT "id", "created_at" FROM "invoices" UNION ALL SELECT "id", "created_at" FROM "credit_notes" ORDER BY "created_at" DESC LIMIT 10
// SELECT * FROM (SELECT "id", "created_at" FROM "invoices" UNION ALL SELECT "id", "created_at" FROM "credit_notes") ORDER BY "created_at" DESC LIMIT 10
```

On ClickHouse, `Union` renders `UNION DISTINCT` (ClickHouse rejects a bare `UNION` unless the
`union_default_mode` setting is set), and a compound with an outer `ORDER BY`/`LIMIT`/`OFFSET` is
wrapped as `SELECT * FROM (...)`, because ClickHouse would otherwise apply those clauses to the last
member only.

A member may not have its own `ORDER BY`, `LIMIT`, `OFFSET`, `WITH`, `SETTINGS` or `UNION`
(`ErrCompoundPart`). `IsCompound()` reports whether a builder has members.

`Where`, `Having` and `Prewhere` called on a compound builder apply to the **first member only**,
not to the combined result:

```go
q := gohan.Select("id").From("a").
	Union(gohan.Select("id").From("b")).
	Where(gohan.Col("id").Gt(10))

sql, args, err := q.Build(gohan.Postgres())
// sql: SELECT "id" FROM "a" WHERE "id" > $1 UNION SELECT "id" FROM "b"
// args: [10]
```

To filter the whole result, wrap the compound in a subquery:

```go
u := gohan.Select("id").From("a").Union(gohan.Select("id").From("b"))

sql, args, err := gohan.From(u.As("u")).Where(gohan.Col("id").Gt(10)).Build(gohan.Postgres())
// sql: SELECT * FROM (SELECT "id" FROM "a" UNION SELECT "id" FROM "b") AS "u" WHERE "id" > $1
// args: [10]
```
