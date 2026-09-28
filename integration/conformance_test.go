package integration

import (
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
