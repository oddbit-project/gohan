// Package integration runs gohan's rendered SQL against real PostgreSQL,
// SQLite and ClickHouse servers. Unit tests in the root module compare
// rendered strings; these tests prove the databases accept that SQL and
// return the intended result.
package integration

import (
	"database/sql"
	"os"
	"strings"
	"testing"

	"github.com/oddbit-project/gohan"
	"github.com/stretchr/testify/require"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

// engine pairs a gohan.Dialect with an open connection to the database it
// targets.
type engine struct {
	name    string
	dialect gohan.Dialect
	db      *sql.DB
}

// engines returns the engines to run this test against: SQLite always (a
// fresh in-memory database), PostgreSQL if GOHAN_PG_DSN is set, ClickHouse
// if GOHAN_CH_DSN is set. GOHAN_REQUIRE_ENGINES names engines (comma
// separated) that must be present; a required engine whose DSN is unset
// fails the test immediately, so CI cannot silently skip an engine.
func engines(t *testing.T) []engine {
	t.Helper()

	var es []engine

	sqliteDB, err := sql.Open("sqlite", "file:"+t.Name()+"?mode=memory&cache=shared")
	require.NoError(t, err)
	sqliteDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqliteDB.Close()) })
	es = append(es, engine{name: "sqlite", dialect: gohan.SQLite(), db: sqliteDB})

	if dsn := os.Getenv("GOHAN_PG_DSN"); dsn != "" {
		pgDB, err := sql.Open("pgx", dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, pgDB.Close()) })
		es = append(es, engine{name: "postgres", dialect: gohan.Postgres(), db: pgDB})
	}

	if dsn := os.Getenv("GOHAN_CH_DSN"); dsn != "" {
		chDB, err := sql.Open("clickhouse", dsn)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, chDB.Close()) })
		es = append(es, engine{name: "clickhouse", dialect: gohan.ClickHouse(), db: chDB})
	}

	if req := os.Getenv("GOHAN_REQUIRE_ENGINES"); req != "" {
		have := make(map[string]bool, len(es))
		for _, e := range es {
			have[e.name] = true
		}
		for _, name := range strings.Split(req, ",") {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			if !have[name] {
				t.Fatalf("GOHAN_REQUIRE_ENGINES requires %q but its DSN is not set", name)
			}
		}
	}

	return es
}

// exec builds st for e.dialect and runs it, failing the test on either a
// build or a database error.
func exec(t *testing.T, e engine, st gohan.Statement) {
	t.Helper()
	query, args, err := st.Build(e.dialect)
	require.NoError(t, err)
	_, err = e.db.Exec(query, args...)
	require.NoErrorf(t, err, "sql: %s args: %v", query, args)
}

// query builds st for e.dialect, runs it and returns every row with every
// column scanned into a string ("NULL" for null), so assertions stay
// engine-independent.
func query(t *testing.T, e engine, st gohan.Statement) [][]string {
	t.Helper()
	q, args, err := st.Build(e.dialect)
	require.NoError(t, err)
	rows, err := e.db.Query(q, args...)
	require.NoErrorf(t, err, "sql: %s args: %v", q, args)
	defer rows.Close()

	cols, err := rows.Columns()
	require.NoError(t, err)

	var out [][]string
	for rows.Next() {
		vals := make([]sql.NullString, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		require.NoError(t, rows.Scan(ptrs...))
		row := make([]string, len(cols))
		for i, v := range vals {
			if v.Valid {
				row[i] = v.String
			} else {
				row[i] = "NULL"
			}
		}
		out = append(out, row)
	}
	require.NoError(t, rows.Err())
	return out
}

// queryErr builds and runs st, returning the build or database error — for
// "the engine rejects this" assertions in later plans.
func queryErr(e engine, st gohan.Statement) error {
	q, args, err := st.Build(e.dialect)
	if err != nil {
		return err
	}
	rows, err := e.db.Query(q, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
	}
	return rows.Err()
}

// ddl runs the engine's hand-written DDL string: pg for postgres, sqlite
// for sqlite, ch for clickhouse.
func ddl(t *testing.T, e engine, pg, sqlite, ch string) {
	t.Helper()
	var stmt string
	switch e.name {
	case "postgres":
		stmt = pg
	case "sqlite":
		stmt = sqlite
	case "clickhouse":
		stmt = ch
	default:
		t.Fatalf("ddl: unknown engine %q", e.name)
	}
	_, err := e.db.Exec(stmt)
	require.NoErrorf(t, err, "ddl: %s", stmt)
}
