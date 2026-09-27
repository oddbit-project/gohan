// This file proves PostgreSQL accepts every row-locking clause gohan can
// render (plans/002-row-locking.md), that SQLite and ClickHouse reject
// them at Build with gohan.ErrUnsupported, and the SKIP LOCKED/NOWAIT
// behaviour: a second, concurrent transaction skips a row the first
// transaction is holding, and NOWAIT fails immediately instead of
// blocking on a row a live transaction still holds.
package integration

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/oddbit-project/gohan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lockableTable creates a table with (id, state) sized for the
// row-locking cases below and returns its name.
func lockableTable(t *testing.T, e engine) string {
	t.Helper()
	tbl := newTable(t, e)
	ddl(t, e,
		"CREATE TABLE "+tbl+" (id integer, state text)",
		"CREATE TABLE "+tbl+" (id integer, state text)",
		"CREATE TABLE "+tbl+" (id Int64, state String) ENGINE = MergeTree ORDER BY id")
	return tbl
}

// TestLockUnsupportedOnNonPostgres asserts that Build rejects every lock
// clause with ErrUnsupported on SQLite and ClickHouse, without touching
// the database.
func TestLockUnsupportedOnNonPostgres(t *testing.T) {
	for _, e := range engines(t) {
		if e.name == "postgres" {
			continue
		}
		e := e
		t.Run(e.name, func(t *testing.T) {
			_, _, err := gohan.From("t").ForUpdate().SkipLocked().Build(e.dialect)
			assert.True(t, errors.Is(err, gohan.ErrUnsupported), "got %v", err)
		})
	}

	// ClickHouseNamed is not in the shared engine harness (it needs a
	// dedicated *sql.DB — see clickhouse_named_test.go), but it must
	// behave like ClickHouse for locking.
	_, _, err := gohan.From("t").ForUpdate().Build(gohan.ClickHouseNamed())
	assert.True(t, errors.Is(err, gohan.ErrUnsupported), "got %v", err)
}

// TestLockGoldenExecutes runs every golden statement from lock_test.go's
// TestLockGolden (root module) against a real PostgreSQL transaction and
// checks it executes without error. Other engines are skipped (already
// covered by TestLockUnsupportedOnNonPostgres above).
func TestLockGoldenExecutes(t *testing.T) {
	var e engine
	found := false
	for _, cand := range engines(t) {
		if cand.name == "postgres" {
			e = cand
			found = true
		}
	}
	if !found {
		t.Skip("postgres engine not configured")
	}

	tbl := lockableTable(t, e)
	exec(t, e, gohan.Insert(tbl).Columns("id", "state").Values(1, "new"))
	exec(t, e, gohan.Insert(tbl).Columns("id", "state").Values(2, "new"))

	other := newTable(t, e)
	ddl(t, e, "CREATE TABLE "+other+" (id integer, t_id integer)", "", "")
	exec(t, e, gohan.Insert(other).Columns("id", "t_id").Values(1, 1))

	cases := []struct {
		name  string
		build gohan.Statement
	}{
		{"for update skip locked", gohan.From(tbl).Where(gohan.Col("state").Eq("new")).OrderBy("id").Limit(1).ForUpdate().SkipLocked()},
		{"for no key update", gohan.From(tbl).Where(gohan.Col("state").Eq("new")).OrderBy("id").Limit(1).ForNoKeyUpdate().SkipLocked()},
		{"for share", gohan.From(tbl).Where(gohan.Col("state").Eq("new")).OrderBy("id").Limit(1).ForShare().SkipLocked()},
		{"for key share", gohan.From(tbl).Where(gohan.Col("state").Eq("new")).OrderBy("id").Limit(1).ForKeyShare().SkipLocked()},
		{"for update nowait", gohan.From(tbl).Where(gohan.Col("state").Eq("new")).OrderBy("id").Limit(1).ForUpdate().NoWait()},
		{"of multiple clauses", gohan.From(gohan.Table(tbl).As("x")).
			Join(gohan.Table(other).As("u"), gohan.Table("u").Col("t_id").Eq(gohan.Table("x").Col("id"))).
			ForUpdate("x").ForShare("u")},
		{"limit offset lock", gohan.From(tbl).Limit(2).Offset(0).ForUpdate()},
		{"lock inside cte", func() *gohan.SelectBuilder {
			locked := gohan.From(tbl).Where(gohan.Col("id").Eq(1)).ForUpdate()
			return gohan.Select("id").With("c", locked).From("c")
		}()},
		{"lock inside in subquery", func() *gohan.SelectBuilder {
			locked := gohan.Select("id").From(tbl).Where(gohan.Col("state").Eq("new")).ForUpdate()
			return gohan.From(tbl + "_outer").Where(gohan.Col("id").In(locked))
		}()},
	}

	// lock_inside_in_subquery references a second table for the outer
	// query; create it.
	outer := tbl + "_outer"
	ddl(t, e, "CREATE TABLE "+outer+" (id integer)", "", "")
	t.Cleanup(func() { _, _ = e.db.Exec("DROP TABLE IF EXISTS " + outer) })

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			tx, err := e.db.Begin()
			require.NoError(t, err)
			defer tx.Rollback()

			q, args, err := c.build.Build(e.dialect)
			require.NoError(t, err)
			_, err = tx.Query(q, args...)
			require.NoErrorf(t, err, "sql: %s args: %v", q, args)
		})
	}
}

// TestLockSkipLockedAndNoWait seeds a jobs table with ids 1..3 and proves:
//  1. tx A locks id 1 with FOR UPDATE SKIP LOCKED; tx B, running the same
//     query concurrently, skips the row tx A holds and gets id 2.
//  2. tx B then tries FOR UPDATE NOWAIT on id 1 (still held by tx A) and
//     gets an immediate lock error instead of blocking.
func TestLockSkipLockedAndNoWait(t *testing.T) {
	var e engine
	found := false
	for _, cand := range engines(t) {
		if cand.name == "postgres" {
			e = cand
			found = true
		}
	}
	if !found {
		t.Skip("postgres engine not configured")
	}

	tbl := lockableTable(t, e)
	exec(t, e, gohan.Insert(tbl).Columns("id", "state").Values(1, "new"))
	exec(t, e, gohan.Insert(tbl).Columns("id", "state").Values(2, "new"))
	exec(t, e, gohan.Insert(tbl).Columns("id", "state").Values(3, "new"))

	txA, err := e.db.Begin()
	require.NoError(t, err)
	defer txA.Rollback()

	txB, err := e.db.Begin()
	require.NoError(t, err)
	defer txB.Rollback()
	// fail instead of hanging if SKIP LOCKED ever stops skipping tx A's row
	_, err = txB.Exec("SET LOCAL lock_timeout = '5s'")
	require.NoError(t, err)

	pick := gohan.From(tbl).Where(gohan.Col("state").Eq("new")).OrderBy("id").Limit(1).ForUpdate().SkipLocked()
	q, args, err := pick.Build(e.dialect)
	require.NoError(t, err)

	idA := queryOneID(t, txA, q, args)
	assert.Equal(t, 1, idA)

	idB := queryOneID(t, txB, q, args)
	assert.Equal(t, 2, idB)

	nowaitQ, nowaitArgs, err := gohan.From(tbl).Where(gohan.Col("id").Eq(1)).ForUpdate().NoWait().Build(e.dialect)
	require.NoError(t, err)

	rows, err := txB.Query(nowaitQ, nowaitArgs...)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
		}
		err = rows.Err()
	}
	require.Error(t, err)
	assert.Contains(t, err.Error(), "could not obtain lock")
}

// queryOneID runs q against tx and returns the single integer column of
// its single row.
func queryOneID(t *testing.T, tx *sql.Tx, q string, args []any) int {
	t.Helper()
	rows, err := tx.Query(q, args...)
	require.NoError(t, err)
	defer rows.Close()
	require.True(t, rows.Next())
	var id int
	var state string
	require.NoError(t, rows.Scan(&id, &state))
	require.NoError(t, rows.Err())
	return id
}
