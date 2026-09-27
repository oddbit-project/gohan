// Command postgres runs gohan against PostgreSQL through database/sql and
// the pgx stdlib driver. The connection string is read from GOHAN_PG_DSN;
// the example creates and finally drops a table named gohan_example_tasks.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/oddbit-project/gohan"
)

// Task maps the gohan_example_tasks table.
type Task struct {
	ID     int64  `db:"id" auto:"true"`
	Title  string `db:"title"`
	Status string `db:"status"`
	Meta   string `db:"meta"` // jsonb, sent as text
}

const table = "gohan_example_tasks"

func main() {
	dsn := os.Getenv("GOHAN_PG_DSN")
	if dsn == "" {
		log.Fatal("set GOHAN_PG_DSN, e.g. postgres://user:password@localhost:5432/dbname?sslmode=disable")
	}
	ctx := context.Background()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	d, err := gohan.DialectFor("pgx")
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
		id     bigserial PRIMARY KEY,
		title  text  NOT NULL UNIQUE,
		status text  NOT NULL,
		meta   jsonb NOT NULL DEFAULT '{}'
	)`)
	defer mustExec(ctx, db, "DROP TABLE "+tbl)

	// INSERT several structs and read the generated ids back.
	query, args := build(d, "insert returning", gohan.Insert(table).
		Rows(
			Task{Title: "write docs", Status: "done", Meta: `{"urgent": true}`},
			Task{Title: "add examples", Status: "open", Meta: `{}`},
			Task{Title: "tag release", Status: "open", Meta: `{"urgent": false}`},
		).
		Returning("id", "title"))
	queryRows(ctx, db, query, args, func(rows *sql.Rows) error {
		var id int64
		var title string
		if err := rows.Scan(&id, &title); err != nil {
			return err
		}
		fmt.Printf("  -> %d %s\n", id, title)
		return nil
	})

	// Upsert: the title already exists, so its status is updated instead.
	exec(ctx, db, d, "upsert", gohan.Insert(table).
		Columns("title", "status").
		Values("add examples", "done").
		OnConflict("title").
		DoUpdateExcluded("status"))

	// Aggregate with CASE: Int keeps the constants typed as integers (a
	// bound $n here would be sent as text and SUM(text) does not exist).
	query, args = build(d, "summary", gohan.Select(
		gohan.CountAll().As("total"),
		gohan.Sum(gohan.Case().
			When(gohan.Col("status").Eq("done"), gohan.Int(1)).
			Else(gohan.Int(0))).As("done"),
	).From(table))
	var total, done int64
	if err := db.QueryRowContext(ctx, query, args...).Scan(&total, &done); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("  -> %d of %d tasks done\n", done, total)

	// jsonb's key-exists operator is a bare '?': Raw writes it as '??'.
	// ILIKE is PostgreSQL's case-insensitive LIKE.
	query, args = build(d, "select", gohan.Select("id", "title", "status").
		From(table).
		Where(
			gohan.Raw("meta ?? ?", "urgent"),
			gohan.Col("title").ILike("%DOCS%"),
		).
		OrderBy("id"))
	queryRows(ctx, db, query, args, func(rows *sql.Rows) error {
		var t Task
		if err := rows.Scan(&t.ID, &t.Title, &t.Status); err != nil {
			return err
		}
		fmt.Printf("  -> %+v\n", t)
		return nil
	})

	// UPDATE ... RETURNING.
	query, args = build(d, "update returning", gohan.Update(table).
		Set("status", "cancelled").
		Where(gohan.Col("status").Eq("open")).
		Returning("id"))
	queryRows(ctx, db, query, args, func(rows *sql.Rows) error {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err
		}
		fmt.Println("  -> cancelled", id)
		return nil
	})

	// DELETE with an IN list expanded from a slice.
	exec(ctx, db, d, "delete", gohan.Delete(table).
		Where(gohan.Col("status").In([]string{"cancelled", "archived"})))
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
	res, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		log.Fatalf("%s: %v", label, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("  -> rows affected:", n)
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
