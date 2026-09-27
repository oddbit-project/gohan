package gohan_test

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/oddbit-project/gohan"
)

// ExampleSelect builds a SELECT with a WHERE, ORDER BY and LIMIT clause
// against the PostgreSQL dialect.
func ExampleSelect() {
	sql, args, err := gohan.Select("id", "name").
		From("users").
		Where(gohan.Col("active").Eq(true)).
		OrderBy(gohan.Col("name").Asc()).
		Limit(10).
		Build(gohan.Postgres())
	fmt.Println(sql, args, err)
	// Output:
	// SELECT "id", "name" FROM "users" WHERE "active" = $1 ORDER BY "name" ASC LIMIT 10 [true] <nil>
}

// ExampleInsert_upsert inserts a row and, on a conflicting primary key,
// updates the other columns from the values that were about to be
// inserted (DoUpdateExcluded).
func ExampleInsert_upsert() {
	sql, args, err := gohan.Insert("users").
		Columns("id", "name", "email").
		Values(1, "alice", "alice@example.com").
		OnConflict("id").
		DoUpdateExcluded("name", "email").
		Build(gohan.Postgres())
	fmt.Println(sql, args, err)
	// Output:
	// INSERT INTO "users" ("id", "name", "email") VALUES ($1, $2, $3) ON CONFLICT ("id") DO UPDATE SET "name" = excluded."name", "email" = excluded."email" [1 alice alice@example.com] <nil>
}

// ExampleUpdate builds an UPDATE. A nil or trivially-true WHERE is
// rejected; see ExampleDelete_requiresWhere.
func ExampleUpdate() {
	sql, args, err := gohan.Update("users").
		Set("name", "bob").
		Where(gohan.Col("id").Eq(1)).
		Build(gohan.Postgres())
	fmt.Println(sql, args, err)
	// Output:
	// UPDATE "users" SET "name" = $1 WHERE "id" = $2 [bob 1] <nil>
}

// ExampleDelete_requiresWhere shows that a DELETE with no WHERE clause
// fails at Build with ErrNoWhere instead of deleting every row. Deleting
// every row on purpose uses All().
func ExampleDelete_requiresWhere() {
	sql, args, err := gohan.Delete("users").Build(gohan.Postgres())
	fmt.Println(sql, args, err)
	// Output:
	// [] gohan: statement requires a WHERE clause; call All() to affect every row
}

// ExampleRaw shows Raw's two placeholder forms with a real PostgreSQL
// operator that itself uses `?`: jsonb's key-exists operator. `??` writes
// a literal `?` (the operator) without consuming an argument; `?` binds
// the argument in position.
func ExampleRaw() {
	sql, args, err := gohan.Select().
		From("docs").
		Where(gohan.Raw("data ?? ?", "owner")).
		Build(gohan.Postgres())
	fmt.Println(sql, args, err)
	// Output:
	// SELECT * FROM "docs" WHERE (data ? $1) [owner] <nil>
}

// ExampleSelectBuilder_Union shows the UNION keyword difference between
// PostgreSQL (UNION) and ClickHouse (UNION DISTINCT).
func ExampleSelectBuilder_Union() {
	q := gohan.Select("id").From("t1").
		Union(gohan.Select("id").From("t2"))

	sqlPg, _, errPg := q.Build(gohan.Postgres())
	fmt.Println(sqlPg, errPg)

	sqlCh, _, errCh := q.Build(gohan.ClickHouse())
	fmt.Println(sqlCh, errCh)
	// Output:
	// SELECT "id" FROM "t1" UNION SELECT "id" FROM "t2" <nil>
	// SELECT "id" FROM "t1" UNION DISTINCT SELECT "id" FROM "t2" <nil>
}

// ExampleCase shows Int for an integer constant inside a CASE expression:
// on PostgreSQL, a bound constant here would be typed as text and fail
// SUM's type check ("function sum(text) does not exist"), so integer
// constants use Int, not a bound value.
func ExampleCase() {
	sql, args, err := gohan.Select(
		gohan.Sum(gohan.Case().
			When(gohan.Col("status").Eq("done"), gohan.Int(1)).
			Else(gohan.Int(0))).As("done_count"),
	).From("tasks").Build(gohan.Postgres())
	fmt.Println(sql, args, err)
	// Output:
	// SELECT SUM(CASE WHEN "status" = $1 THEN 1 ELSE 0 END) AS "done_count" FROM "tasks" [done] <nil>
}

// ---------------------------------------------------------------------
// SELECT
// ---------------------------------------------------------------------

// ExampleFrom is shorthand for Select().From(table): with no select list
// the statement selects every column.
func ExampleFrom() {
	sql, args, err := gohan.From("public.users").
		Where(gohan.Col("id").Eq(42)).
		Build(gohan.Postgres())
	fmt.Println(sql, args, err)
	// Output:
	// SELECT * FROM "public"."users" WHERE "id" = $1 [42] <nil>
}

// ExampleSelectBuilder_Where shows that repeated Where calls are ANDed,
// and that Or/And nest with explicit parentheses.
func ExampleSelectBuilder_Where() {
	sql, args, err := gohan.Select("id", "total").
		From("orders").
		Where(gohan.Col("status").Eq("paid")).
		Where(gohan.Or(
			gohan.Col("total").Gte(100),
			gohan.Col("customer_tier").In("gold", "platinum"),
		)).
		Build(gohan.Postgres())
	fmt.Println(sql)
	fmt.Println(args, err)
	// Output:
	// SELECT "id", "total" FROM "orders" WHERE ("status" = $1 AND ("total" >= $2 OR "customer_tier" IN ($3, $4)))
	// [paid 100 gold platinum] <nil>
}

// ExampleSelectBuilder_OrderBy mixes a plain column name (rendered
// ascending) with an explicit Desc().NullsLast() item.
func ExampleSelectBuilder_OrderBy() {
	sql, _, err := gohan.From("users").
		OrderBy("last_name", gohan.Col("last_login").Desc().NullsLast()).
		Build(gohan.Postgres())
	fmt.Println(sql, err)
	// Output:
	// SELECT * FROM "users" ORDER BY "last_name" ASC, "last_login" DESC NULLS LAST <nil>
}

// ExampleSelectBuilder_Limit renders LIMIT and OFFSET inline (they are
// integers, not bound values).
func ExampleSelectBuilder_Limit() {
	sql, args, err := gohan.From("users").
		OrderBy("id").
		Limit(20).
		Offset(40).
		Build(gohan.Postgres())
	fmt.Println(sql, args, err)
	// Output:
	// SELECT * FROM "users" ORDER BY "id" ASC LIMIT 20 OFFSET 40 [] <nil>
}

// ExampleSelectBuilder_Offset shows that an OFFSET without a LIMIT gets
// "LIMIT -1" on SQLite, whose grammar has no bare OFFSET.
func ExampleSelectBuilder_Offset() {
	q := gohan.From("users").Offset(40)

	sqlPg, _, _ := q.Build(gohan.Postgres())
	fmt.Println(sqlPg)

	sqlLite, _, _ := q.Build(gohan.SQLite())
	fmt.Println(sqlLite)
	// Output:
	// SELECT * FROM "users" OFFSET 40
	// SELECT * FROM `users` LIMIT -1 OFFSET 40
}

// ExampleSelectBuilder_Distinct adds DISTINCT to the select list.
func ExampleSelectBuilder_Distinct() {
	sql, _, err := gohan.Select("country").From("customers").Distinct().Build(gohan.Postgres())
	fmt.Println(sql, err)
	// Output:
	// SELECT DISTINCT "country" FROM "customers" <nil>
}

// ExampleSelectBuilder_Join joins two aliased tables. TableRef.Col
// qualifies a column by the table's alias.
func ExampleSelectBuilder_Join() {
	u := gohan.Table("users").As("u")
	o := gohan.Table("orders").As("o")

	sql, args, err := gohan.Select(u.Col("name"), o.Col("total")).
		From(u).
		Join(o, o.Col("user_id").Eq(u.Col("id"))).
		Where(o.Col("total").Gt(100)).
		Build(gohan.Postgres())
	fmt.Println(sql)
	fmt.Println(args, err)
	// Output:
	// SELECT "u"."name", "o"."total" FROM "users" AS "u" INNER JOIN "orders" AS "o" ON "o"."user_id" = "u"."id" WHERE "o"."total" > $1
	// [100] <nil>
}

// ExampleSelectBuilder_LeftJoin keeps every row of the left table.
func ExampleSelectBuilder_LeftJoin() {
	sql, _, err := gohan.Select("users.id", gohan.Count("orders.id").As("orders")).
		From("users").
		LeftJoin("orders", gohan.Col("orders.user_id").Eq(gohan.Col("users.id"))).
		GroupBy("users.id").
		Build(gohan.SQLite())
	fmt.Println(sql, err)
	// Output:
	// SELECT `users`.`id`, COUNT(`orders`.`id`) AS `orders` FROM `users` LEFT JOIN `orders` ON `orders`.`user_id` = `users`.`id` GROUP BY `users`.`id` <nil>
}

// ExampleSelectBuilder_JoinUsing matches rows on identically named
// columns.
func ExampleSelectBuilder_JoinUsing() {
	sql, _, err := gohan.From("orders").
		JoinUsing("customers", "customer_id").
		Build(gohan.Postgres())
	fmt.Println(sql, err)
	// Output:
	// SELECT * FROM "orders" INNER JOIN "customers" USING ("customer_id") <nil>
}

// ExampleSelectBuilder_GroupBy aggregates with GROUP BY and filters groups
// with HAVING. Aggregate arguments given as strings are column names.
func ExampleSelectBuilder_GroupBy() {
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
	fmt.Println(sql)
	fmt.Println(args, err)
	// Output:
	// SELECT "customer_id", COUNT(*) AS "orders", SUM("total") AS "revenue" FROM "orders" GROUP BY "customer_id" HAVING SUM("total") > $1 ORDER BY SUM("total") DESC
	// [1000] <nil>
}

// ExampleSelectBuilder_As turns a SELECT into an aliased subquery usable
// as a FROM (or JOIN) source. Placeholders are numbered across the whole
// statement.
func ExampleSelectBuilder_As() {
	recent := gohan.Select("user_id", gohan.Max("created_at").As("last_order")).
		From("orders").
		Where(gohan.Col("status").Eq("paid")).
		GroupBy("user_id").
		As("r")

	sql, args, err := gohan.From(recent).
		Where(gohan.Col("r.last_order").Lt("2024-01-01")).
		Build(gohan.Postgres())
	fmt.Println(sql)
	fmt.Println(args, err)
	// Output:
	// SELECT * FROM (SELECT "user_id", MAX("created_at") AS "last_order" FROM "orders" WHERE "status" = $1 GROUP BY "user_id") AS "r" WHERE "r"."last_order" < $2
	// [paid 2024-01-01] <nil>
}

// ExampleSelectBuilder_With adds a common table expression.
func ExampleSelectBuilder_With() {
	active := gohan.Select("id", "email").From("users").Where(gohan.Col("active").Eq(true))

	sql, args, err := gohan.Select("email").
		With("active_users", active).
		From("active_users").
		Where(gohan.Col("email").HasSuffix("@example.com")).
		Build(gohan.Postgres())
	fmt.Println(sql)
	fmt.Println(args, err)
	// Output:
	// WITH "active_users" AS (SELECT "id", "email" FROM "users" WHERE "active" = $1) SELECT "email" FROM "active_users" WHERE "email" LIKE $2 ESCAPE '!'
	// [true %@example.com] <nil>
}

// ExampleSelectBuilder_WithRecursive walks a tree with a recursive CTE.
func ExampleSelectBuilder_WithRecursive() {
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
	fmt.Println(sql)
	fmt.Println(args, err)
	// Output:
	// WITH RECURSIVE "tree" AS (SELECT "id", "parent_id" FROM "categories" WHERE "id" = $1 UNION ALL SELECT "c"."id", "c"."parent_id" FROM "categories" AS "c" INNER JOIN "tree" ON "c"."parent_id" = "tree"."id") SELECT "id" FROM "tree"
	// [1] <nil>
}

// ExampleSelectBuilder_UnionAll shows an outer ORDER BY / LIMIT on a
// compound. ClickHouse wraps the compound in a subquery, because its
// UNION grammar would bind the trailing clauses to the last member only.
func ExampleSelectBuilder_UnionAll() {
	q := gohan.Select("id", "created_at").From("invoices").
		UnionAll(gohan.Select("id", "created_at").From("credit_notes")).
		OrderBy(gohan.Col("created_at").Desc()).
		Limit(10)

	sqlPg, _, _ := q.Build(gohan.Postgres())
	fmt.Println(sqlPg)

	sqlCh, _, _ := q.Build(gohan.ClickHouse())
	fmt.Println(sqlCh)
	// Output:
	// SELECT "id", "created_at" FROM "invoices" UNION ALL SELECT "id", "created_at" FROM "credit_notes" ORDER BY "created_at" DESC LIMIT 10
	// SELECT * FROM (SELECT "id", "created_at" FROM "invoices" UNION ALL SELECT "id", "created_at" FROM "credit_notes") ORDER BY "created_at" DESC LIMIT 10
}

// ExampleSelectBuilder_IsCompound shows the restriction on UNION members:
// a member may not carry its own ORDER BY, LIMIT, OFFSET, WITH, SETTINGS
// or UNION.
func ExampleSelectBuilder_IsCompound() {
	q := gohan.Select("id").From("a").
		Union(gohan.Select("id").From("b").Limit(5))
	fmt.Println(q.IsCompound())

	_, _, err := q.Build(gohan.Postgres())
	fmt.Println(errors.Is(err, gohan.ErrCompoundPart))
	// Output:
	// true
	// true
}

// ---------------------------------------------------------------------
// ClickHouse-specific SELECT clauses
// ---------------------------------------------------------------------

// ExampleSelectBuilder_Final reads a ReplacingMergeTree table with FINAL.
// On a dialect without FeatureClickHouse the clause fails with
// ErrUnsupported.
func ExampleSelectBuilder_Final() {
	q := gohan.From("events").Final().Where(gohan.Col("tenant_id").Eq(7))

	sql, args, err := q.Build(gohan.ClickHouse())
	fmt.Println(sql, args, err)

	_, _, err = q.Build(gohan.Postgres())
	fmt.Println(errors.Is(err, gohan.ErrUnsupported))
	// Output:
	// SELECT * FROM "events" FINAL WHERE "tenant_id" = ? [7] <nil>
	// true
}

// ExampleSelectBuilder_Sample reads a sample of a table: Sample takes a
// ratio in (0, 1], SampleRows an absolute row count.
func ExampleSelectBuilder_Sample() {
	sql, _, err := gohan.Select(gohan.CountAll()).From("hits").Sample(0.1).Build(gohan.ClickHouse())
	fmt.Println(sql, err)

	sql, _, err = gohan.Select(gohan.CountAll()).From("hits").SampleRows(10000).Build(gohan.ClickHouse())
	fmt.Println(sql, err)

	_, _, err = gohan.From("hits").Sample(1.5).Build(gohan.ClickHouse())
	fmt.Println(err)
	// Output:
	// SELECT COUNT(*) FROM "hits" SAMPLE 0.1 <nil>
	// SELECT COUNT(*) FROM "hits" SAMPLE 10000 <nil>
	// gohan: SAMPLE ratio must be in (0, 1]
}

// ExampleSelectBuilder_ArrayJoin unfolds an array column into rows.
func ExampleSelectBuilder_ArrayJoin() {
	sql, _, err := gohan.Select("id", "tag").
		From("articles").
		ArrayJoin(gohan.Col("tags").As("tag")).
		Build(gohan.ClickHouse())
	fmt.Println(sql, err)
	// Output:
	// SELECT "id", "tag" FROM "articles" ARRAY JOIN "tags" AS "tag" <nil>
}

// ExampleSelectBuilder_Prewhere filters with PREWHERE before reading the
// remaining columns.
func ExampleSelectBuilder_Prewhere() {
	sql, args, err := gohan.Select("url", "duration").
		From("requests").
		Prewhere(gohan.Col("date").Eq("2024-06-01")).
		Where(gohan.Col("duration").Gt(1000)).
		Build(gohan.ClickHouse())
	fmt.Println(sql)
	fmt.Println(args, err)
	// Output:
	// SELECT "url", "duration" FROM "requests" PREWHERE "date" = ? WHERE "duration" > ?
	// [2024-06-01 1000] <nil>
}

// ExampleSelectBuilder_Settings appends a SETTINGS clause. Keys are
// validated and written in sorted order; values are bound.
func ExampleSelectBuilder_Settings() {
	sql, args, err := gohan.From("events").
		Limit(100).
		Settings(map[string]any{"max_threads": 4, "join_use_nulls": 1}).
		Build(gohan.ClickHouse())
	fmt.Println(sql)
	fmt.Println(args, err)
	// Output:
	// SELECT * FROM "events" LIMIT 100 SETTINGS join_use_nulls = ?, max_threads = ?
	// [1 4] <nil>
}

// ---------------------------------------------------------------------
// Expressions
// ---------------------------------------------------------------------

// ExampleCol shows the string-position rule: a string on the right-hand
// side of a comparison is a bound value; Col makes it a column.
func ExampleCol() {
	asValue, args1, _ := gohan.From("t").Where(gohan.Col("a").Eq("b")).Build(gohan.Postgres())
	fmt.Println(asValue, args1)

	asColumn, args2, _ := gohan.From("t").Where(gohan.Col("a").Eq(gohan.Col("b"))).Build(gohan.Postgres())
	fmt.Println(asColumn, args2)
	// Output:
	// SELECT * FROM "t" WHERE "a" = $1 [b]
	// SELECT * FROM "t" WHERE "a" = "b" []
}

// ExampleVal binds a value where a column name would otherwise be
// assumed, here as the left-hand side of IN. Val(nil) renders NULL.
func ExampleVal() {
	sql, args, err := gohan.From("users").
		Where(gohan.Val("admin").In(gohan.Col("role"), gohan.Col("backup_role"))).
		Build(gohan.Postgres())
	fmt.Println(sql, args, err)

	sql, args, err = gohan.Select(gohan.Val(nil).As("nothing")).Build(gohan.Postgres())
	fmt.Println(sql, args, err)
	// Output:
	// SELECT * FROM "users" WHERE $1 IN ("role", "backup_role") [admin] <nil>
	// SELECT NULL AS "nothing" [] <nil>
}

// ExampleValue_Eq shows that comparing with nil renders IS NULL / IS NOT
// NULL instead of binding a NULL (which would never match).
func ExampleValue_Eq() {
	sql, args, err := gohan.From("users").
		Where(gohan.Col("deleted_at").Eq(nil), gohan.Col("email").Neq(nil)).
		Build(gohan.Postgres())
	fmt.Println(sql, args, err)
	// Output:
	// SELECT * FROM "users" WHERE ("deleted_at" IS NULL AND "email" IS NOT NULL) [] <nil>
}

// ExampleValue_Between renders an inclusive range.
func ExampleValue_Between() {
	sql, args, err := gohan.From("orders").
		Where(gohan.Col("total").Between(10, 100)).
		Build(gohan.SQLite())
	fmt.Println(sql, args, err)
	// Output:
	// SELECT * FROM `orders` WHERE `total` BETWEEN ? AND ? [10 100] <nil>
}

// ExampleAnd shows AND grouping; And() with no arguments is the always-true
// (1=1).
func ExampleAnd() {
	sql, args, err := gohan.From("t").
		Where(gohan.Or(
			gohan.And(gohan.Col("a").Eq(1), gohan.Col("b").Eq(2)),
			gohan.Col("c").Eq(3),
		)).
		Build(gohan.Postgres())
	fmt.Println(sql, args, err)

	sql, _, _ = gohan.From("t").Where(gohan.And()).Build(gohan.Postgres())
	fmt.Println(sql)
	// Output:
	// SELECT * FROM "t" WHERE (("a" = $1 AND "b" = $2) OR "c" = $3) [1 2 3] <nil>
	// SELECT * FROM "t" WHERE (1=1)
}

// ExampleOr shows OR grouping; Or() with no arguments is the always-false
// (1=0).
func ExampleOr() {
	sql, args, err := gohan.From("t").
		Where(gohan.Or(gohan.Col("a").Eq(1), gohan.Col("a").IsNull())).
		Build(gohan.SQLite())
	fmt.Println(sql, args, err)

	sql, _, _ = gohan.From("t").Where(gohan.Or()).Build(gohan.SQLite())
	fmt.Println(sql)
	// Output:
	// SELECT * FROM `t` WHERE (`a` = ? OR `a` IS NULL) [1] <nil>
	// SELECT * FROM `t` WHERE (1=0)
}

// ExampleNot negates an expression.
func ExampleNot() {
	sql, args, err := gohan.From("t").
		Where(gohan.Not(gohan.Or(gohan.Col("a").Eq(1), gohan.Col("b").Eq(2)))).
		Build(gohan.Postgres())
	fmt.Println(sql, args, err)
	// Output:
	// SELECT * FROM "t" WHERE NOT (("a" = $1 OR "b" = $2)) [1 2] <nil>
}

// ExampleValue_In expands a single slice argument into one placeholder
// per element; an empty list renders the always-false 1=0.
func ExampleValue_In() {
	ids := []int{3, 5, 8}
	sql, args, err := gohan.From("users").Where(gohan.Col("id").In(ids)).Build(gohan.Postgres())
	fmt.Println(sql, args, err)

	sql, args, err = gohan.From("users").Where(gohan.Col("id").In([]int{})).Build(gohan.Postgres())
	fmt.Println(sql, args, err)
	// Output:
	// SELECT * FROM "users" WHERE "id" IN ($1, $2, $3) [3 5 8] <nil>
	// SELECT * FROM "users" WHERE 1=0 [] <nil>
}

// ExampleValue_In_subquery passes a *SelectBuilder to In. The subquery
// shares the outer statement's placeholder numbering.
func ExampleValue_In_subquery() {
	banned := gohan.Select("user_id").From("bans").Where(gohan.Col("reason").Eq("spam"))

	sql, args, err := gohan.From("comments").
		Where(gohan.Col("user_id").In(banned), gohan.Col("hidden").Eq(false)).
		Build(gohan.Postgres())
	fmt.Println(sql)
	fmt.Println(args, err)
	// Output:
	// SELECT * FROM "comments" WHERE ("user_id" IN (SELECT "user_id" FROM "bans" WHERE "reason" = $1) AND "hidden" = $2)
	// [spam false] <nil>
}

// ExampleValue_NotIn shows NOT IN; an empty list renders the always-true
// 1=1.
func ExampleValue_NotIn() {
	sql, args, err := gohan.From("users").Where(gohan.Col("status").NotIn("banned", "deleted")).Build(gohan.SQLite())
	fmt.Println(sql, args, err)

	sql, args, err = gohan.From("users").Where(gohan.Col("status").NotIn()).Build(gohan.SQLite())
	fmt.Println(sql, args, err)
	// Output:
	// SELECT * FROM `users` WHERE `status` NOT IN (?, ?) [banned deleted] <nil>
	// SELECT * FROM `users` WHERE 1=1 [] <nil>
}

// ExampleValue_Like binds the pattern as-is: wildcards in it are live.
// Use Contains/HasPrefix/HasSuffix for untrusted search text.
func ExampleValue_Like() {
	sql, args, err := gohan.From("users").Where(gohan.Col("email").Like("%@example.com")).Build(gohan.Postgres())
	fmt.Println(sql, args, err)
	// Output:
	// SELECT * FROM "users" WHERE "email" LIKE $1 [%@example.com] <nil>
}

// ExampleValue_Contains escapes LIKE wildcards in the search text, so
// "50%_off" matches literally. The escape character differs between
// ClickHouse (backslash) and the other dialects (ESCAPE '!').
func ExampleValue_Contains() {
	q := gohan.From("products").Where(gohan.Col("name").Contains("50%_off"))

	for _, d := range []gohan.Dialect{gohan.Postgres(), gohan.SQLite(), gohan.ClickHouse()} {
		sql, args, err := q.Build(d)
		fmt.Println(sql, args, err)
	}
	// Output:
	// SELECT * FROM "products" WHERE "name" LIKE $1 ESCAPE '!' [%50!%!_off%] <nil>
	// SELECT * FROM `products` WHERE `name` LIKE ? ESCAPE '!' [%50!%!_off%] <nil>
	// SELECT * FROM "products" WHERE "name" LIKE ? [%50\%\_off%] <nil>
}

// ExampleValue_HasPrefix matches a literal prefix.
func ExampleValue_HasPrefix() {
	sql, args, err := gohan.From("files").Where(gohan.Col("path").HasPrefix("/tmp/")).Build(gohan.Postgres())
	fmt.Println(sql, args, err)
	// Output:
	// SELECT * FROM "files" WHERE "path" LIKE $1 ESCAPE '!' [/tmp/%] <nil>
}

// ExampleValue_ILike is available on PostgreSQL and ClickHouse only
// (FeatureILike); SQLite's LIKE is already ASCII case-insensitive.
func ExampleValue_ILike() {
	q := gohan.From("users").Where(gohan.Col("name").ILike("ann%"))

	sql, args, err := q.Build(gohan.Postgres())
	fmt.Println(sql, args, err)

	_, _, err = q.Build(gohan.SQLite())
	fmt.Println(errors.Is(err, gohan.ErrUnsupported), err)
	// Output:
	// SELECT * FROM "users" WHERE "name" ILIKE $1 [ann%] <nil>
	// true gohan: not supported by dialect: ILIKE
}

// ExampleMatch builds an AND of equality comparisons from a map, with
// keys in sorted order.
func ExampleMatch() {
	sql, args, err := gohan.From("users").
		Where(gohan.Match(map[string]any{"status": "active", "country": "PT"})).
		Build(gohan.Postgres())
	fmt.Println(sql, args, err)
	// Output:
	// SELECT * FROM "users" WHERE ("country" = $1 AND "status" = $2) [PT active] <nil>
}

// ExampleExists filters with a correlated EXISTS subquery.
func ExampleExists() {
	sql, args, err := gohan.From(gohan.Table("users").As("u")).
		Where(gohan.Exists(
			gohan.Select(gohan.Int(1)).From("orders").
				Where(gohan.Col("orders.user_id").Eq(gohan.Col("u.id")), gohan.Col("orders.total").Gt(500)),
		)).
		Build(gohan.Postgres())
	fmt.Println(sql)
	fmt.Println(args, err)
	// Output:
	// SELECT * FROM "users" AS "u" WHERE EXISTS (SELECT 1 FROM "orders" WHERE ("orders"."user_id" = "u"."id" AND "orders"."total" > $1))
	// [500] <nil>
}

// ExampleNotExists filters with NOT EXISTS.
func ExampleNotExists() {
	sql, _, err := gohan.From(gohan.Table("users").As("u")).
		Where(gohan.NotExists(
			gohan.Select(gohan.Int(1)).From("orders").
				Where(gohan.Col("orders.user_id").Eq(gohan.Col("u.id"))),
		)).
		Build(gohan.SQLite())
	fmt.Println(sql, err)
	// Output:
	// SELECT * FROM `users` AS `u` WHERE NOT EXISTS (SELECT 1 FROM `orders` WHERE `orders`.`user_id` = `u`.`id`) <nil>
}

// ExampleSub compares against a scalar subquery.
func ExampleSub() {
	avg := gohan.Select(gohan.Avg("total")).From("orders").Where(gohan.Col("status").Eq("paid"))

	sql, args, err := gohan.Select("id", "total").
		From("orders").
		Where(gohan.Col("total").Gt(gohan.Sub(avg))).
		Build(gohan.Postgres())
	fmt.Println(sql)
	fmt.Println(args, err)
	// Output:
	// SELECT "id", "total" FROM "orders" WHERE "total" > (SELECT AVG("total") FROM "orders" WHERE "status" = $1)
	// [paid] <nil>
}

// ExampleFn calls a SQL function. The function name is trusted input and
// is validated against ^[A-Za-z_][A-Za-z0-9_]*$; arguments are in value
// position, so pass Col for a column.
func ExampleFn() {
	sql, args, err := gohan.From("users").
		Where(gohan.Fn("lower", gohan.Col("email")).Eq("alice@example.com")).
		Build(gohan.Postgres())
	fmt.Println(sql, args, err)

	_, _, err = gohan.Select(gohan.Fn("lower(email); --")).From("users").Build(gohan.Postgres())
	fmt.Println(errors.Is(err, gohan.ErrInvalidFunction))
	// Output:
	// SELECT * FROM "users" WHERE lower("email") = $1 [alice@example.com] <nil>
	// true
}

// ExampleCast gives a bound value a concrete type. The type string is
// trusted input and is validated.
func ExampleCast() {
	sql, args, err := gohan.Select(gohan.Cast("2024-01-01", "date").As("d")).Build(gohan.Postgres())
	fmt.Println(sql, args, err)

	sql, args, err = gohan.Select(gohan.Cast(gohan.Col("price"), "Decimal(10, 2)")).From("items").Build(gohan.ClickHouse())
	fmt.Println(sql, args, err)
	// Output:
	// SELECT CAST($1 AS date) AS "d" [2024-01-01] <nil>
	// SELECT CAST("price" AS Decimal(10, 2)) FROM "items" [] <nil>
}

// ExampleInt renders an integer literal inline instead of binding it;
// negative values are parenthesized.
func ExampleInt() {
	sql, args, err := gohan.Select(gohan.Int(1).As("one"), gohan.Int(-5).As("minus_five")).Build(gohan.Postgres())
	fmt.Println(sql, args, err)
	// Output:
	// SELECT 1 AS "one", (-5) AS "minus_five" [] <nil>
}

// ExampleCaseBuilder_End finishes a CASE without an ELSE branch. THEN
// values are in value position, so strings are bound.
func ExampleCaseBuilder_End() {
	sql, args, err := gohan.Select(
		"id",
		gohan.Case().
			When(gohan.Col("score").Gte(90), "A").
			When(gohan.Col("score").Gte(80), "B").
			End().As("grade"),
	).From("exams").Build(gohan.Postgres())
	fmt.Println(sql)
	fmt.Println(args, err)
	// Output:
	// SELECT "id", CASE WHEN "score" >= $1 THEN $2 WHEN "score" >= $3 THEN $4 END AS "grade" FROM "exams"
	// [90 A 80 B] <nil>
}

// ExampleTable names a table with an alias; TableRef.Col qualifies a
// column by that alias.
func ExampleTable() {
	u := gohan.Table("app.users").As("u")
	sql, args, err := gohan.Select(u.Col("id"), u.Col("name")).From(u).Where(u.Col("id").Eq(1)).Build(gohan.Postgres())
	fmt.Println(sql, args, err)
	// Output:
	// SELECT "u"."id", "u"."name" FROM "app"."users" AS "u" WHERE "u"."id" = $1 [1] <nil>
}

// ExampleValue_As aliases an expression in the select list.
func ExampleValue_As() {
	sql, _, err := gohan.Select(gohan.Fn("upper", gohan.Col("name")).As("name_uc")).From("users").Build(gohan.SQLite())
	fmt.Println(sql, err)
	// Output:
	// SELECT upper(`name`) AS `name_uc` FROM `users` <nil>
}

// ---------------------------------------------------------------------
// INSERT
// ---------------------------------------------------------------------

// ExampleInsert inserts a single row from explicit columns and values.
func ExampleInsert() {
	sql, args, err := gohan.Insert("users").
		Columns("name", "email").
		Values("alice", "alice@example.com").
		Build(gohan.SQLite())
	fmt.Println(sql, args, err)
	// Output:
	// INSERT INTO `users` (`name`, `email`) VALUES (?, ?) [alice alice@example.com] <nil>
}

// ExampleInsertBuilder_Values appends one row per call; the value count
// must match the column count.
func ExampleInsertBuilder_Values() {
	sql, args, err := gohan.Insert("users").
		Columns("name", "email").
		Values("alice", "alice@example.com").
		Values("bob", "bob@example.com").
		Build(gohan.Postgres())
	fmt.Println(sql)
	fmt.Println(args, err)

	_, _, err = gohan.Insert("users").Columns("name", "email").Values("carol").Build(gohan.Postgres())
	fmt.Println(errors.Is(err, gohan.ErrValueCount))
	// Output:
	// INSERT INTO "users" ("name", "email") VALUES ($1, $2), ($3, $4)
	// [alice alice@example.com bob bob@example.com] <nil>
	// true
}

// ExampleInsertBuilder_Rows inserts structs. The first record decides the
// columns: auto fields (here the id) are skipped, and a nil pointer with
// goqu:"omitnil" is left out.
func ExampleInsertBuilder_Rows() {
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
	fmt.Println(sql)
	fmt.Println(args, err)
	// Output:
	// INSERT INTO "users" ("name", "email") VALUES ($1, $2), ($3, $4)
	// [alice alice@example.com bob bob@example.com] <nil>
}

// ExampleInsertBuilder_SetMap inserts one row from a map, with columns in
// sorted key order.
func ExampleInsertBuilder_SetMap() {
	sql, args, err := gohan.Insert("settings").
		SetMap(map[string]any{"key": "theme", "value": "dark"}).
		Build(gohan.Postgres())
	fmt.Println(sql, args, err)
	// Output:
	// INSERT INTO "settings" ("key", "value") VALUES ($1, $2) [theme dark] <nil>
}

// ExampleInsertBuilder_FromSelect copies rows with INSERT ... SELECT.
func ExampleInsertBuilder_FromSelect() {
	sql, args, err := gohan.Insert("archived_orders").
		Columns("id", "total").
		FromSelect(gohan.Select("id", "total").From("orders").Where(gohan.Col("created_at").Lt("2020-01-01"))).
		Build(gohan.Postgres())
	fmt.Println(sql)
	fmt.Println(args, err)
	// Output:
	// INSERT INTO "archived_orders" ("id", "total") SELECT "id", "total" FROM "orders" WHERE "created_at" < $1
	// [2020-01-01] <nil>
}

// ExampleInsertBuilder_FromSelect_sqliteUpsert shows the "WHERE true"
// that SQLite needs between an INSERT ... SELECT and its ON CONFLICT
// clause; gohan adds it when the SELECT has no WHERE of its own.
func ExampleInsertBuilder_FromSelect_sqliteUpsert() {
	sql, _, err := gohan.Insert("tags").
		Columns("name").
		FromSelect(gohan.Select("name").From("staging_tags")).
		OnConflict("name").DoNothing().
		Build(gohan.SQLite())
	fmt.Println(sql, err)
	// Output:
	// INSERT INTO `tags` (`name`) SELECT `name` FROM `staging_tags` WHERE true ON CONFLICT (`name`) DO NOTHING <nil>
}

// ExampleInsertBuilder_Returning returns generated columns.
func ExampleInsertBuilder_Returning() {
	sql, args, err := gohan.Insert("users").
		Columns("name").
		Values("alice").
		Returning("id", "created_at").
		Build(gohan.Postgres())
	fmt.Println(sql, args, err)
	// Output:
	// INSERT INTO "users" ("name") VALUES ($1) RETURNING "id", "created_at" [alice] <nil>
}

// ExampleConflictBuilder_DoNothing ignores rows that violate a unique
// constraint.
func ExampleConflictBuilder_DoNothing() {
	sql, args, err := gohan.Insert("tags").
		Columns("name").
		Values("go").
		OnConflict("name").DoNothing().
		Build(gohan.SQLite())
	fmt.Println(sql, args, err)
	// Output:
	// INSERT INTO `tags` (`name`) VALUES (?) ON CONFLICT (`name`) DO NOTHING [go] <nil>
}

// ExampleConflictBuilder_DoUpdate sets explicit values on conflict: an
// Expr such as Excluded or Raw renders in place, anything else is bound.
// Keys are written in sorted order.
func ExampleConflictBuilder_DoUpdate() {
	sql, args, err := gohan.Insert("page_views").
		Columns("path", "views", "last_seen").
		Values("/home", 1, "2024-06-01").
		OnConflict("path").
		DoUpdate(map[string]any{
			"views":     gohan.Raw("? + 1", gohan.Col("page_views.views")),
			"last_seen": gohan.Excluded("last_seen"),
		}).
		Build(gohan.Postgres())
	fmt.Println(sql)
	fmt.Println(args, err)
	// Output:
	// INSERT INTO "page_views" ("path", "views", "last_seen") VALUES ($1, $2, $3) ON CONFLICT ("path") DO UPDATE SET "last_seen" = excluded."last_seen", "views" = "page_views"."views" + 1
	// [/home 1 2024-06-01] <nil>
}

// ---------------------------------------------------------------------
// UPDATE
// ---------------------------------------------------------------------

// ExampleUpdateBuilder_SetMap sets several columns from a map, in sorted
// key order. A value that is an Expr renders in place.
func ExampleUpdateBuilder_SetMap() {
	sql, args, err := gohan.Update("accounts").
		SetMap(map[string]any{
			"balance":    gohan.Raw("balance - ?", 25),
			"updated_by": "system",
		}).
		Where(gohan.Col("id").Eq(9)).
		Build(gohan.Postgres())
	fmt.Println(sql, args, err)
	// Output:
	// UPDATE "accounts" SET "balance" = balance - $1, "updated_by" = $2 WHERE "id" = $3 [25 system 9] <nil>
}

// ExampleUpdateBuilder_SetRecord writes a struct's updatable fields. Auto
// fields are skipped; IncludeFields/ExcludeFields accept either the Go
// field name or the column name.
func ExampleUpdateBuilder_SetRecord() {
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
	fmt.Println(sql, args, err)

	sql, args, err = gohan.Update("users").
		SetRecord(User{Name: "bob"}, gohan.SkipZeroValues()).
		Where(gohan.Col("id").Eq(8)).
		Build(gohan.Postgres())
	fmt.Println(sql, args, err)
	// Output:
	// UPDATE "users" SET "name" = $1, "email" = $2 WHERE "id" = $3 [alice alice@example.com 7] <nil>
	// UPDATE "users" SET "name" = $1 WHERE "id" = $2 [bob 8] <nil>
}

// ExampleUpdateBuilder_Where shows the trivially-true guard: a WHERE that
// can only be true (here an empty NotIn) is refused like a missing one.
func ExampleUpdateBuilder_Where() {
	_, _, err := gohan.Update("users").
		Set("active", false).
		Where(gohan.Col("id").NotIn()).
		Build(gohan.Postgres())
	fmt.Println(errors.Is(err, gohan.ErrNoWhere))
	// Output:
	// true
}

// ExampleUpdateBuilder_All updates every row on purpose.
func ExampleUpdateBuilder_All() {
	sql, args, err := gohan.Update("users").Set("active", false).All().Build(gohan.SQLite())
	fmt.Println(sql, args, err)
	// Output:
	// UPDATE `users` SET `active` = ? [false] <nil>
}

// ExampleUpdate_clickHouse shows that the ClickHouse dialect does not
// build UPDATE statements (it lacks FeatureUpdate).
func ExampleUpdate_clickHouse() {
	_, _, err := gohan.Update("events").Set("x", 1).Where(gohan.Col("id").Eq(1)).Build(gohan.ClickHouse())
	fmt.Println(errors.Is(err, gohan.ErrUnsupported), err)
	// Output:
	// true gohan: not supported by dialect: UPDATE
}

// ---------------------------------------------------------------------
// DELETE
// ---------------------------------------------------------------------

// ExampleDelete deletes matching rows and returns their ids.
func ExampleDelete() {
	sql, args, err := gohan.Delete("sessions").
		Where(gohan.Col("expires_at").Lt("2024-01-01")).
		Returning("id").
		Build(gohan.SQLite())
	fmt.Println(sql, args, err)
	// Output:
	// DELETE FROM `sessions` WHERE `expires_at` < ? RETURNING `id` [2024-01-01] <nil>
}

// ExampleDeleteBuilder_All deletes every row on purpose. ClickHouse's
// lightweight DELETE requires a WHERE, so it gets "WHERE 1".
func ExampleDeleteBuilder_All() {
	q := gohan.Delete("scratch").All()

	sqlPg, _, _ := q.Build(gohan.Postgres())
	fmt.Println(sqlPg)

	sqlCh, _, _ := q.Build(gohan.ClickHouse())
	fmt.Println(sqlCh)
	// Output:
	// DELETE FROM "scratch"
	// DELETE FROM "scratch" WHERE 1
}

// ExampleDeleteBuilder_Where shows that the trivially-true guard looks
// through Not/Or, but is per statement: a real condition ANDed with a
// trivially-true one is accepted.
func ExampleDeleteBuilder_Where() {
	_, _, err := gohan.Delete("users").Where(gohan.Not(gohan.Or())).Build(gohan.Postgres())
	fmt.Println(errors.Is(err, gohan.ErrNoWhere))

	sql, args, err := gohan.Delete("users").
		Where(gohan.Col("tenant_id").Eq(3), gohan.Col("status").NotIn()).
		Build(gohan.Postgres())
	fmt.Println(sql, args, err)
	// Output:
	// true
	// DELETE FROM "users" WHERE ("tenant_id" = $1 AND 1=1) [3] <nil>
}

// ---------------------------------------------------------------------
// Errors
// ---------------------------------------------------------------------

// ExampleError shows that every failure is reported by Build as an error
// wrapping one of the Err* constants, for use with errors.Is.
func ExampleError() {
	_, _, err := gohan.Delete("users").Build(gohan.Postgres())
	fmt.Println(errors.Is(err, gohan.ErrNoWhere))

	_, _, err = gohan.Select("a?b").From("t").Build(gohan.ClickHouse())
	fmt.Println(errors.Is(err, gohan.ErrInvalidIdentifier), err)

	_, _, err = gohan.From("t").Where(gohan.Col("attrs").Eq(map[string]string{"k": "v"})).Build(gohan.ClickHouse())
	fmt.Println(errors.Is(err, gohan.ErrUnsafeValue), err)

	_, _, err = gohan.From("t").Where(gohan.Raw("a = ? AND b = ?", 1)).Build(gohan.Postgres())
	fmt.Println(errors.Is(err, gohan.ErrRawArgs))

	_, err = gohan.DialectFor("mysql")
	fmt.Println(errors.Is(err, gohan.ErrUnknownDialect), err)
	// Output:
	// true
	// true gohan: invalid identifier: "a?b"
	// true gohan: value type cannot be bound safely for this dialect: map[string]string
	// true
	// true gohan: unknown or zero dialect: "mysql"
}

// ---------------------------------------------------------------------
// Dialects
// ---------------------------------------------------------------------

// ExampleDialect renders one query in every built-in dialect: identifier
// quoting and placeholder style differ, the bound arguments do not.
func ExampleDialect() {
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
}

// ExampleDialectFor looks a dialect up by database/sql driver name.
func ExampleDialectFor() {
	for _, driver := range []string{"pgx", "sqlite", "clickhouse"} {
		d, err := gohan.DialectFor(driver)
		fmt.Println(driver, "->", d.Name(), err)
	}
	// Output:
	// pgx -> postgres <nil>
	// sqlite -> sqlite <nil>
	// clickhouse -> clickhouse <nil>
}

// ExampleRegister maps another driver name to a built-in dialect.
func ExampleRegister() {
	gohan.Register("libsql", gohan.SQLite())

	d, err := gohan.DialectFor("libsql")
	fmt.Println(d.Name(), err)
	// Output:
	// sqlite <nil>
}

// ExampleDialect_Has reports optional features and the bound-argument
// limit of each dialect.
func ExampleDialect_Has() {
	for _, d := range []gohan.Dialect{gohan.Postgres(), gohan.SQLite(), gohan.ClickHouse(), gohan.Generic()} {
		fmt.Printf("%s returning=%v upsert=%v ilike=%v update=%v clickhouse=%v maxargs=%d\n",
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
	// postgres returning=true upsert=true ilike=true update=true clickhouse=false maxargs=65535
	// sqlite returning=true upsert=true ilike=false update=true clickhouse=false maxargs=32766
	// clickhouse returning=false upsert=false ilike=true update=false clickhouse=true maxargs=0
	// generic returning=false upsert=false ilike=false update=true clickhouse=false maxargs=999
}

// ExampleDialect_QuoteIdent quotes an identifier outside a statement
// (e.g. for DDL). A dot separates qualified parts; quote characters inside
// a part are escaped by doubling.
func ExampleDialect_QuoteIdent() {
	for _, d := range []gohan.Dialect{gohan.Postgres(), gohan.SQLite(), gohan.ClickHouse()} {
		q, err := d.QuoteIdent(`audit."log"`)
		fmt.Println(d.Name()+":", q, err)
	}
	// Output:
	// postgres: "audit"."""log""" <nil>
	// sqlite: `audit`.`"log"` <nil>
	// clickhouse: "audit"."""log""" <nil>
}

// ---------------------------------------------------------------------
// Records
// ---------------------------------------------------------------------

// ExampleRecordColumns lists every column a struct type maps, auto fields
// included (e.g. for a SELECT list).
func ExampleRecordColumns() {
	type User struct {
		ID       int64  `db:"id" auto:"true"`
		Name     string `db:"name"`
		Email    string `db:"email"`
		Password string `db:"-"`
	}

	cols, err := gohan.RecordColumns(reflect.TypeOf(User{}))
	fmt.Println(cols, err)

	sql, _, err := gohan.Select(toAny(cols)...).From("users").Build(gohan.Postgres())
	fmt.Println(sql, err)
	// Output:
	// [id name email] <nil>
	// SELECT "id", "name", "email" FROM "users" <nil>
}

// toAny converts a []string into the []any that Select accepts.
func toAny(cols []string) []any {
	out := make([]any, len(cols))
	for i, c := range cols {
		out[i] = c
	}
	return out
}

// ExampleInsertColumns lists the columns an INSERT of a record would
// write: auto fields, nil omitnil pointers and zero omitempty values are
// left out.
func ExampleInsertColumns() {
	type Post struct {
		ID    int64   `db:"id" auto:"true"`
		Title string  `db:"title"`
		Body  string  `db:"body" goqu:"omitempty"`
		Slug  *string `db:"slug" goqu:"omitnil"`
	}

	cols, err := gohan.InsertColumns(Post{Title: "hello"})
	fmt.Println(cols, err)

	slug := "hello"
	cols, err = gohan.InsertColumns(Post{Title: "hello", Body: "...", Slug: &slug})
	fmt.Println(cols, err)
	// Output:
	// [title] <nil>
	// [title body slug] <nil>
}
