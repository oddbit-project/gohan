// Command clickhouse runs gohan against ClickHouse through database/sql
// and clickhouse-go v2. The connection string is read from GOHAN_CH_DSN;
// the example creates and finally drops a table named
// gohan_example_events.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/oddbit-project/gohan"
)

const table = "gohan_example_events"

func main() {
	dsn := os.Getenv("GOHAN_CH_DSN")
	if dsn == "" {
		log.Fatal("set GOHAN_CH_DSN, e.g. clickhouse://user:password@localhost:9000/dbname")
	}
	ctx := context.Background()

	db, err := sql.Open("clickhouse", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	d, err := gohan.DialectFor("clickhouse")
	if err != nil {
		log.Fatal(err)
	}

	// DDL is written by hand; QuoteIdent quotes the table name for it.
	tbl, err := d.QuoteIdent(table)
	if err != nil {
		log.Fatal(err)
	}
	mustExec(ctx, db, "DROP TABLE IF EXISTS "+tbl)
	mustExec(ctx, db, `CREATE TABLE `+tbl+` (
		event_id UInt64,
		tenant   UInt32,
		user_id  UInt64,
		kind     String,
		tags     Array(String),
		version  UInt32
	) ENGINE = ReplacingMergeTree(version)
	ORDER BY (tenant, intHash32(user_id), event_id)
	SAMPLE BY intHash32(user_id)`)
	defer mustExec(ctx, db, "DROP TABLE "+tbl)

	// A multi-row INSERT. Event 1 is written twice: ReplacingMergeTree
	// keeps the row with the highest version once parts are merged.
	exec(ctx, db, d, "insert", gohan.Insert(table).
		Columns("event_id", "tenant", "user_id", "kind", "tags", "version").
		Values(1, 7, 100, "click", []string{"web", "promo"}, 1).
		Values(1, 7, 100, "purchase", []string{"web", "promo"}, 2).
		Values(2, 7, 101, "click", []string{"app"}, 1).
		Values(3, 7, 102, "view", []string{"web"}, 1).
		Values(4, 8, 100, "view", []string{"app"}, 1))

	// FINAL collapses the duplicate at read time; PREWHERE filters on the
	// sorting key before the other columns are read; SETTINGS is bound.
	query, args := build(d, "select final", gohan.Select("event_id", "kind").
		From(table).
		Final().
		Prewhere(gohan.Col("tenant").Eq(7)).
		Where(gohan.Col("kind").NotIn("view")).
		OrderBy("event_id").
		Settings(map[string]any{"max_threads": 2}))
	queryRows(ctx, db, query, args, func(rows *sql.Rows) error {
		var id uint64
		var kind string
		if err := rows.Scan(&id, &kind); err != nil {
			return err
		}
		fmt.Println("  ->", id, kind)
		return nil
	})

	// ARRAY JOIN unfolds the tags array into one row per tag.
	query, args = build(d, "array join", gohan.Select("tag", gohan.CountAll().As("n")).
		From(table).
		Final().
		ArrayJoin(gohan.Col("tags").As("tag")).
		GroupBy("tag").
		OrderBy("tag"))
	queryRows(ctx, db, query, args, func(rows *sql.Rows) error {
		var tag string
		var n uint64
		if err := rows.Scan(&tag, &n); err != nil {
			return err
		}
		fmt.Println("  ->", tag, n)
		return nil
	})

	// SAMPLE reads a deterministic fraction of users (the table's SAMPLE
	// BY key).
	query, args = build(d, "sample", gohan.Select(gohan.Count("user_id").As("n")).
		From(table).
		Sample(0.5))
	var sampled uint64
	if err := db.QueryRowContext(ctx, query, args...).Scan(&sampled); err != nil {
		log.Fatal(err)
	}
	fmt.Println("  -> rows in sample:", sampled)

	// Lightweight DELETE. UPDATE is not available on this dialect.
	exec(ctx, db, d, "delete", gohan.Delete(table).Where(gohan.Col("tenant").Eq(8)))
	if _, _, err := gohan.Update(table).Set("kind", "x").Where(gohan.Col("tenant").Eq(7)).Build(d); err != nil {
		fmt.Println("\nupdate:", err)
	}

	// A map value cannot be bound safely by clickhouse-go and is refused.
	if _, _, err := gohan.From(table).Where(gohan.Col("kind").Eq(map[string]string{"k": "v"})).Build(d); err != nil {
		fmt.Println("map value:", err)
	}

	query, args = build(d, "count", gohan.Select(gohan.CountAll()).From(table).Final())
	var remaining uint64
	if err := db.QueryRowContext(ctx, query, args...).Scan(&remaining); err != nil {
		log.Fatal(err)
	}
	fmt.Println("  -> remaining events:", remaining)
}

// build renders stmt under d and prints it.
func build(d gohan.Dialect, label string, stmt gohan.Statement) (string, []any) {
	query, args, err := stmt.Build(d)
	if err != nil {
		log.Fatalf("%s: %v", label, err)
	}
	fmt.Printf("\n%s:\n  %s\n  args: %v\n", label, query, args)
	return query, args
}

// exec builds and runs a statement that returns no rows.
func exec(ctx context.Context, db *sql.DB, d gohan.Dialect, label string, stmt gohan.Statement) {
	query, args := build(d, label, stmt)
	if _, err := db.ExecContext(ctx, query, args...); err != nil {
		log.Fatalf("%s: %v", label, err)
	}
	fmt.Println("  -> ok")
}

// queryRows runs query and calls scan for every row.
func queryRows(ctx context.Context, db *sql.DB, query string, args []any, scan func(*sql.Rows) error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		if err := scan(rows); err != nil {
			log.Fatal(err)
		}
	}
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}
}

// mustExec runs hand-written DDL.
func mustExec(ctx context.Context, db *sql.DB, query string) {
	if _, err := db.ExecContext(ctx, query); err != nil {
		log.Fatal(err)
	}
}
