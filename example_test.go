package gohan_test

import (
	"fmt"

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
