// This file proves gohan's INSERT ... DEFAULT VALUES (plans/011) against
// real PostgreSQL and SQLite servers, and that SQLite rejects DEFAULT
// VALUES combined with ON CONFLICT and ClickHouse rejects DEFAULT VALUES
// outright, both at Build with gohan.ErrUnsupported.
package integration

import (
	"errors"
	"testing"

	"github.com/oddbit-project/gohan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// defaultValuesTable creates a table (id, code, name) whose columns all
// take a default (measured against real PostgreSQL/SQLite behaviour in
// plans/011-insert-default-values.md) and returns its name.
func defaultValuesTable(t *testing.T, e engine) string {
	t.Helper()
	tbl := newTable(t, e)
	ddl(t, e,
		"CREATE TABLE "+tbl+" (id serial PRIMARY KEY, code int DEFAULT 1 UNIQUE, name text DEFAULT 'dflt')",
		"CREATE TABLE "+tbl+" (id INTEGER PRIMARY KEY, code integer DEFAULT 1 UNIQUE, name text DEFAULT 'dflt')",
		"")
	return tbl
}

// TestDefaultValuesInsertReturning asserts that DEFAULT VALUES inserts one
// row taking every column's default, on PostgreSQL and SQLite.
func TestDefaultValuesInsertReturning(t *testing.T) {
	for _, e := range engines(t) {
		if e.name == "clickhouse" {
			continue
		}
		e := e
		t.Run(e.name, func(t *testing.T) {
			tbl := defaultValuesTable(t, e)

			rows := query(t, e, gohan.Insert(tbl).DefaultValues().Returning("id", "code", "name"))
			require.Len(t, rows, 1)
			assert.Equal(t, []string{"1", "1", "dflt"}, rows[0])
		})
	}
}

// TestDefaultValuesOnConflictDoNothing asserts that a second DEFAULT
// VALUES insert conflicting on the unique "code" column affects 0 rows on
// PostgreSQL.
func TestDefaultValuesOnConflictDoNothing(t *testing.T) {
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

	tbl := defaultValuesTable(t, e)
	exec(t, e, gohan.Insert(tbl).DefaultValues())

	q, args, err := gohan.Insert(tbl).DefaultValues().OnConflict("code").DoNothing().Build(e.dialect)
	require.NoError(t, err)
	res, err := e.db.Exec(q, args...)
	require.NoErrorf(t, err, "sql: %s args: %v", q, args)
	n, err := res.RowsAffected()
	require.NoError(t, err)
	assert.Equal(t, int64(0), n)
}

// TestDefaultValuesSQLiteOnConflictUnsupported asserts that Build rejects
// DEFAULT VALUES combined with ON CONFLICT on SQLite, without touching the
// database (SQLite's grammar makes this a syntax error).
func TestDefaultValuesSQLiteOnConflictUnsupported(t *testing.T) {
	for _, e := range engines(t) {
		if e.name != "sqlite" {
			continue
		}
		_, _, err := gohan.Insert("t").DefaultValues().OnConflict("id").DoNothing().Build(e.dialect)
		assert.True(t, errors.Is(err, gohan.ErrUnsupported), "got %v", err)
	}
}

// TestDefaultValuesClickHouseUnsupported asserts that Build rejects
// DEFAULT VALUES on ClickHouse (measured: real ClickHouse returns a
// syntax error for it). ClickHouseNamed is not in the shared engine
// harness, but it must behave the same way.
func TestDefaultValuesClickHouseUnsupported(t *testing.T) {
	_, _, err := gohan.Insert("t").DefaultValues().Build(gohan.ClickHouse())
	assert.True(t, errors.Is(err, gohan.ErrUnsupported), "got %v", err)

	_, _, err = gohan.Insert("t").DefaultValues().Build(gohan.ClickHouseNamed())
	assert.True(t, errors.Is(err, gohan.ErrUnsupported), "got %v", err)
}
