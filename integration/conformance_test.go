package integration

import (
	"database/sql"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/oddbit-project/gohan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tableSeq gives every table created by these tests a unique, unquoted
// name, since PostgreSQL and ClickHouse are shared across the whole test
// binary (unlike the per-call in-memory SQLite database).
var tableSeq atomic.Uint64

// newTable returns a fresh table name and registers a cleanup that drops
// it on e.db.
func newTable(t *testing.T, e engine) string {
	t.Helper()
	name := fmt.Sprintf("t_%d", tableSeq.Add(1))
	t.Cleanup(func() {
		_, _ = e.db.Exec("DROP TABLE IF EXISTS " + name)
	})
	return name
}

// idNameTable creates a table (id, name) sized for the id/name cases
// below and returns its name.
func idNameTable(t *testing.T, e engine) string {
	t.Helper()
	tbl := newTable(t, e)
	ddl(t, e,
		"CREATE TABLE "+tbl+" (id integer, name text)",
		"CREATE TABLE "+tbl+" (id integer, name text)",
		"CREATE TABLE "+tbl+" (id Int64, name String) ENGINE = MergeTree ORDER BY id")
	return tbl
}

func TestBoundValuesRoundTrip(t *testing.T) {
	names := []string{`a'b`, `a\b`, `a"b`, "a\\nb", `x' OR '1'='1`}

	for _, e := range engines(t) {
		e := e
		t.Run(e.name, func(t *testing.T) {
			tbl := idNameTable(t, e)
			for i, n := range names {
				exec(t, e, gohan.Insert(tbl).Columns("id", "name").Values(i, n))
			}

			for _, n := range names {
				rows := query(t, e, gohan.Select("name").From(tbl).Where(gohan.Col("name").Eq(n)))
				require.Len(t, rows, 1, "value %q", n)
				assert.Equal(t, n, rows[0][0])
			}

			rows := query(t, e, gohan.Select("name").From(tbl).Where(gohan.Col("name").Eq("' OR 1=1 --")))
			assert.Empty(t, rows)
		})
	}
}

func TestContainsIsLiteral(t *testing.T) {
	names := []string{`50%`, `5_0`, `a\b`, `plain`}

	for _, e := range engines(t) {
		e := e
		t.Run(e.name, func(t *testing.T) {
			tbl := idNameTable(t, e)
			for i, n := range names {
				exec(t, e, gohan.Insert(tbl).Columns("id", "name").Values(i, n))
			}

			rows := query(t, e, gohan.Select("name").From(tbl).Where(gohan.Col("name").Contains("%")))
			require.Len(t, rows, 1)
			assert.Equal(t, `50%`, rows[0][0])

			rows = query(t, e, gohan.Select("name").From(tbl).Where(gohan.Col("name").Contains("_")))
			require.Len(t, rows, 1)
			assert.Equal(t, `5_0`, rows[0][0])

			rows = query(t, e, gohan.Select("name").From(tbl).Where(gohan.Col("name").Contains(`\`)))
			require.Len(t, rows, 1)
			assert.Equal(t, `a\b`, rows[0][0])
		})
	}
}

func TestFoldHelpers(t *testing.T) {
	names := []string{
		`xxAbC%_!yy`,
		`XXABC%_!YY`,
		`xxAbCZZ!yy`, // matches only if % or _ were live wildcards
		`xxabc%_\yy`,
		`abc%_!`,
		`Abc%_!tail`,
	}

	for _, e := range engines(t) {
		e := e
		t.Run(e.name, func(t *testing.T) {
			tbl := idNameTable(t, e)
			for i, n := range names {
				exec(t, e, gohan.Insert(tbl).Columns("id", "name").Values(i, n))
			}

			ids := func(cond gohan.Expr) []string {
				var out []string
				for _, r := range query(t, e, gohan.Select("id").From(tbl).Where(cond).OrderBy("id")) {
					out = append(out, r[0])
				}
				return out
			}

			assert.Equal(t, []string{"0", "1", "4", "5"}, ids(gohan.Col("name").ContainsFold("aBc%_!")))
			assert.Equal(t, []string{"4", "5"}, ids(gohan.Col("name").HasPrefixFold("ABC%_!")))
			assert.Equal(t, []string{"0", "1"}, ids(gohan.Col("name").HasSuffixFold("aBc%_!YY")))
			assert.Equal(t, []string{"3"}, ids(gohan.Col("name").ContainsFold(`C%_\`)))
			assert.Empty(t, ids(gohan.Col("name").ContainsFold("c_z")))
		})
	}
}

func TestUnionOrderLimit(t *testing.T) {
	for _, e := range engines(t) {
		e := e
		t.Run(e.name, func(t *testing.T) {
			tbl := idNameTable(t, e)
			for i := 1; i <= 4; i++ {
				exec(t, e, gohan.Insert(tbl).Columns("id", "name").Values(i, fmt.Sprintf("n%d", i)))
			}

			rows := query(t, e, gohan.Select("id").From(tbl).Where(gohan.Col("id").Lt(3)).
				Union(gohan.Select("id").From(tbl).Where(gohan.Col("id").Gt(1))).
				OrderBy(gohan.Col("id").Desc()).
				Limit(2))
			require.Len(t, rows, 2)
			assert.Equal(t, "4", rows[0][0])
			assert.Equal(t, "3", rows[1][0])
		})
	}
}

func TestUpsert(t *testing.T) {
	for _, e := range engines(t) {
		e := e
		t.Run(e.name, func(t *testing.T) {
			if e.name == "clickhouse" {
				t.Skip("no upsert")
			}

			tbl := newTable(t, e)
			ddl(t, e,
				"CREATE TABLE "+tbl+" (id integer PRIMARY KEY, name text)",
				"CREATE TABLE "+tbl+" (id integer PRIMARY KEY, name text)",
				"")

			exec(t, e, gohan.Insert(tbl).Columns("id", "name").Values(1, "a"))
			exec(t, e, gohan.Insert(tbl).Columns("id", "name").Values(1, "b").
				OnConflict("id").DoUpdateExcluded("name"))

			rows := query(t, e, gohan.Select("name").From(tbl).Where(gohan.Col("id").Eq(1)))
			require.Len(t, rows, 1)
			assert.Equal(t, "b", rows[0][0])
		})
	}
}

func TestDeleteAll(t *testing.T) {
	for _, e := range engines(t) {
		e := e
		t.Run(e.name, func(t *testing.T) {
			tbl := idNameTable(t, e)
			exec(t, e, gohan.Insert(tbl).Columns("id", "name").Values(1, "a"))
			exec(t, e, gohan.Insert(tbl).Columns("id", "name").Values(2, "b"))

			exec(t, e, gohan.Delete(tbl).All())

			rows := query(t, e, gohan.Select("id").From(tbl))
			assert.Empty(t, rows)
		})
	}
}

func TestQuotedIdentifiers(t *testing.T) {
	for _, e := range engines(t) {
		e := e
		t.Run(e.name, func(t *testing.T) {
			tbl := newTable(t, e)
			ddl(t, e,
				`CREATE TABLE `+tbl+` (id integer, "select" text, "a b" text)`,
				"CREATE TABLE "+tbl+" (id integer, `select` text, `a b` text)",
				`CREATE TABLE `+tbl+` (id Int64, "select" String, "a b" String) ENGINE = MergeTree ORDER BY id`)

			exec(t, e, gohan.Insert(tbl).Columns("id", "select", "a b").Values(1, "x", "y"))

			rows := query(t, e, gohan.Select("select", "a b").From(tbl).Where(gohan.Col("id").Eq(1)))
			require.Len(t, rows, 1)
			assert.Equal(t, []string{"x", "y"}, rows[0])
		})
	}
}

// TestNilPointerDoesNotMatchNull proves, against real PostgreSQL,
// ClickHouse and SQLite, that Eq/Neq with a typed nil pointer bind it as
// an ordinary parameter instead of rendering IS NULL/IS NOT NULL: a row
// whose val column is actually NULL is matched by Eq(nil) but not by
// Eq((*int)(nil)), and is matched by Neq((*int)(nil)) but not by
// Neq(nil).
func TestNilPointerDoesNotMatchNull(t *testing.T) {
	for _, e := range engines(t) {
		e := e
		t.Run(e.name, func(t *testing.T) {
			tbl := newTable(t, e)
			ddl(t, e,
				"CREATE TABLE "+tbl+" (id integer, val integer)",
				"CREATE TABLE "+tbl+" (id integer, val integer)",
				"CREATE TABLE "+tbl+" (id Int64, val Nullable(Int32)) ENGINE = MergeTree ORDER BY id")

			exec(t, e, gohan.Insert(tbl).Columns("id", "val").Values(1, nil))
			exec(t, e, gohan.Insert(tbl).Columns("id", "val").Values(2, 5))

			var nilPtr *int

			// Eq(nil): untyped nil, renders IS NULL, matches the NULL row.
			rows := query(t, e, gohan.Select("id").From(tbl).Where(gohan.Col("val").Eq(nil)))
			require.Len(t, rows, 1)
			assert.Equal(t, "1", rows[0][0])

			// Eq(nilPtr): a nil pointer is bound as a parameter, so
			// "val = NULL" matches no row, including the one with a NULL
			// val.
			rows = query(t, e, gohan.Select("id").From(tbl).Where(gohan.Col("val").Eq(nilPtr)))
			assert.Empty(t, rows)

			// Neq(nil): untyped nil, renders IS NOT NULL, matches the
			// non-NULL row.
			rows = query(t, e, gohan.Select("id").From(tbl).Where(gohan.Col("val").Neq(nil)))
			require.Len(t, rows, 1)
			assert.Equal(t, "2", rows[0][0])

			// Neq(nilPtr): "val <> NULL" matches no row either.
			rows = query(t, e, gohan.Select("id").From(tbl).Where(gohan.Col("val").Neq(nilPtr)))
			assert.Empty(t, rows)
		})
	}
}

// TestNilValuerPointerDoesNotPanic proves that binding a nil pointer whose
// type implements driver.Valuer with a value-receiver Value() method (here
// *sql.NullString) does not panic on any engine and matches no row, now
// that Eq binds a nil pointer as an ordinary parameter instead of
// rendering IS NULL. Without the writer's nilValuer normalization,
// database/sql itself would call Value() on the nil pointer and panic.
func TestNilValuerPointerDoesNotPanic(t *testing.T) {
	for _, e := range engines(t) {
		e := e
		t.Run(e.name, func(t *testing.T) {
			tbl := idNameTable(t, e)
			exec(t, e, gohan.Insert(tbl).Columns("id", "name").Values(1, "a"))

			var p *sql.NullString
			require.NotPanics(t, func() {
				rows := query(t, e, gohan.Select("id").From(tbl).Where(gohan.Col("name").Eq(p)))
				assert.Empty(t, rows)
			})
		})
	}
}
