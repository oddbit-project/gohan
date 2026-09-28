package gohan

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComparisons(t *testing.T) {
	tests := []struct {
		name    string
		dialect Dialect
		expr    Expr
		sql     string
		args    []any
		err     error
	}{
		{"eq", Postgres(), Col("a").Eq(5), `"a" = $1`, []any{5}, nil},
		{"eq nil", Postgres(), Col("a").Eq(nil), `"a" IS NULL`, []any{}, nil},
		{"eq nil ptr", Postgres(), Col("a").Eq((*int)(nil)), `"a" = $1`, []any{(*int)(nil)}, nil},
		{"neq nil", Postgres(), Col("a").Neq(nil), `"a" IS NOT NULL`, []any{}, nil},
		{"neq nil ptr", Postgres(), Col("a").Neq((*int)(nil)), `"a" <> $1`, []any{(*int)(nil)}, nil},
		{"eq col", Postgres(), Col("a").Eq(Col("b")), `"a" = "b"`, []any{}, nil},
		{"in ints", Postgres(), Col("a").In(1, 2), `"a" IN ($1, $2)`, []any{1, 2}, nil},
		{"in slice", Postgres(), Col("a").In([]int{1, 2}), `"a" IN ($1, $2)`, []any{1, 2}, nil},
		{"in empty", Postgres(), Col("a").In(), "1=0", []any{}, nil},
		{"notin empty", Postgres(), Col("a").NotIn(), "1=1", []any{}, nil},
		{"between", Postgres(), Col("a").Between(1, 9), `"a" BETWEEN $1 AND $2`, []any{1, 9}, nil},
		{"and two", Postgres(), And(Col("a").Eq(1), Col("b").Eq(2)), `("a" = $1 AND "b" = $2)`, []any{1, 2}, nil},
		{"and one", Postgres(), And(Col("a").Eq(1)), `"a" = $1`, []any{1}, nil},
		{"and raw", Postgres(), And(Col("t").Eq(1), Raw("a OR b")), `("t" = $1 AND (a OR b))`, []any{1}, nil},
		{"and empty", Postgres(), And(), "(1=1)", []any{}, nil},
		{"or empty", Postgres(), Or(), "(1=0)", []any{}, nil},
		{"not", Postgres(), Not(Col("a").Eq(1)), `NOT ("a" = $1)`, []any{1}, nil},
		{"match", Postgres(), Match(map[string]any{"b": 2, "a": 1}), `("a" = $1 AND "b" = $2)`, []any{1, 2}, nil},
		{"match empty", Postgres(), Match(map[string]any{}), "", nil, ErrEmptyMatch},
		{"val nil", Postgres(), Val(nil), "NULL", []any{}, nil},
		{"val nil ptr", Postgres(), Val((*int)(nil)), "$1", []any{(*int)(nil)}, nil},
		{"int pos", Postgres(), Int(7), "7", []any{}, nil},
		{"int neg", Postgres(), Int(-3), "(-3)", []any{}, nil},
		{"as", Postgres(), Col("a").As("x"), `"a" AS "x"`, []any{}, nil},
		{"order", Postgres(), Col("a").Desc().NullsLast(), `"a" DESC NULLS LAST`, []any{}, nil},
		{"eq sqlite", SQLite(), Col("a").Eq(1), "`a` = ?", []any{1}, nil},
		{"ilike sqlite", SQLite(), Col("a").ILike("x"), "", nil, ErrUnsupported},
		{"unsafe clickhouse", ClickHouse(), Col("a").Eq(map[string]int{"k": 1}), "", nil, ErrUnsafeValue},
		{"neq", Postgres(), Col("a").Neq(5), `"a" <> $1`, []any{5}, nil},
		{"gt", Postgres(), Col("a").Gt(5), `"a" > $1`, []any{5}, nil},
		{"gte", Postgres(), Col("a").Gte(5), `"a" >= $1`, []any{5}, nil},
		{"lt", Postgres(), Col("a").Lt(5), `"a" < $1`, []any{5}, nil},
		{"lte", Postgres(), Col("a").Lte(5), `"a" <= $1`, []any{5}, nil},
		{"notin", Postgres(), Col("a").NotIn(1, 2), `"a" NOT IN ($1, $2)`, []any{1, 2}, nil},
		{"notbetween", Postgres(), Col("a").NotBetween(1, 9), `"a" NOT BETWEEN $1 AND $2`, []any{1, 9}, nil},
		{"like", Postgres(), Col("a").Like("x%"), `"a" LIKE $1`, []any{"x%"}, nil},
		{"notlike", Postgres(), Col("a").NotLike("x%"), `"a" NOT LIKE $1`, []any{"x%"}, nil},
		{"ilike pg", Postgres(), Col("a").ILike("x"), `"a" ILIKE $1`, []any{"x"}, nil},
		{"notilike pg", Postgres(), Col("a").NotILike("x"), `"a" NOT ILIKE $1`, []any{"x"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args, err := render(tt.dialect, tt.expr)
			if tt.err != nil {
				assert.True(t, errors.Is(err, tt.err), "got %v", err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.sql, sql)
			assert.Equal(t, tt.args, args)
		})
	}
}

// TestNilHandling checks that only an untyped nil (including a nil
// interface holding nothing) renders IS NULL; a nil pointer is bound as a
// parameter, whatever its pointee type.
func TestNilHandling(t *testing.T) {
	var nilIface any
	tests := []struct {
		name string
		x    any
	}{
		{"untyped nil", nil},
		{"nil interface", nilIface},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args, err := render(Postgres(), Col("a").Eq(tt.x))
			require.NoError(t, err)
			assert.Equal(t, `"a" IS NULL`, sql)
			assert.Equal(t, []any{}, args)
		})
	}
	n := 5
	sql, args, err := render(Postgres(), Col("a").Eq(&n))
	require.NoError(t, err)
	assert.Equal(t, `"a" = $1`, sql)
	assert.Equal(t, []any{&n}, args)
}

// ptrRecvValuer implements driver.Valuer with a pointer receiver: calling
// Value() on a nil *ptrRecvValuer panics, unlike a value-receiver Valuer
// such as sql.NullString (database/sql's callValuerValue already skips a
// nil pointer whose Value method has a value receiver, and binds NULL
// directly). clickhouse-go calls Value() on a nil pointer of either
// receiver kind, so it would panic for both.
type ptrRecvValuer struct{ n int }

func (v *ptrRecvValuer) Value() (driver.Value, error) { return int64(v.n), nil }

// TestNilPointerBinding checks that a nil pointer is an ordinary value —
// bound as a parameter, not treated as SQL NULL — across Eq, Neq, Val,
// Match, and every dialect, where a nil pointer to a driver.Valuer type
// must go through the writer's nilValuer normalization instead of gohan's
// own NULL rendering. nilValuer normalizes a nil pointer implementing
// driver.Valuer regardless of receiver kind: database/sql itself only
// panics on the pointer-receiver kind, but clickhouse-go panics on either,
// so gohan treats both the same on every dialect for consistency.
func TestNilPointerBinding(t *testing.T) {
	t.Run("eq nil pointer binds, does not render IS NULL", func(t *testing.T) {
		sql, args, err := render(Postgres(), Col("x").Eq((*int)(nil)))
		require.NoError(t, err)
		assert.Equal(t, `"x" = $1`, sql)
		assert.Equal(t, []any{(*int)(nil)}, args)
	})

	t.Run("neq nil pointer binds, does not render IS NOT NULL", func(t *testing.T) {
		sql, args, err := render(Postgres(), Col("x").Neq((*int)(nil)))
		require.NoError(t, err)
		assert.Equal(t, `"x" <> $1`, sql)
		assert.Equal(t, []any{(*int)(nil)}, args)
	})

	t.Run("val nil pointer binds, does not render NULL", func(t *testing.T) {
		sql, args, err := render(Postgres(), Val((*int)(nil)))
		require.NoError(t, err)
		assert.Equal(t, "$1", sql)
		assert.Equal(t, []any{(*int)(nil)}, args)
	})

	t.Run("match with nil pointer field binds", func(t *testing.T) {
		sql, args, err := render(Postgres(), Match(map[string]any{"x": (*int)(nil)}))
		require.NoError(t, err)
		assert.Equal(t, `"x" = $1`, sql)
		assert.Equal(t, []any{(*int)(nil)}, args)
	})

	// sql.NullString.Value has a value receiver: database/sql itself would
	// already bind this nil pointer as NULL without help, but clickhouse-go
	// would call Value() on it and panic. Eq no longer treats this nil
	// pointer as SQL NULL, so it must reach the writer's nilValuer
	// normalization instead of the driver.
	t.Run("eq nil Valuer pointer normalizes to untyped nil on postgres", func(t *testing.T) {
		var p *sql.NullString
		stmt, args, err := render(Postgres(), Col("x").Eq(p))
		require.NoError(t, err)
		assert.Equal(t, `"x" = $1`, stmt)
		assert.Equal(t, []any{nil}, args)
	})

	// ptrRecvValuer has a pointer-receiver Value(): database/sql would call
	// it on the nil pointer and panic without nilValuer's normalization.
	t.Run("eq nil pointer-receiver Valuer normalizes to untyped nil on postgres", func(t *testing.T) {
		var p *ptrRecvValuer
		stmt, args, err := render(Postgres(), Col("x").Eq(p))
		require.NoError(t, err)
		assert.Equal(t, `"x" = $1`, stmt)
		assert.Equal(t, []any{nil}, args)
	})

	t.Run("eq nil Valuer pointer normalizes to untyped nil on sqlite", func(t *testing.T) {
		var p *sql.NullString
		stmt, args, err := render(SQLite(), Col("x").Eq(p))
		require.NoError(t, err)
		assert.Equal(t, "`x` = ?", stmt)
		assert.Equal(t, []any{nil}, args)
	})

	t.Run("eq nil Valuer pointer normalizes to untyped nil on clickhouse", func(t *testing.T) {
		var p *sql.NullString
		stmt, args, err := render(ClickHouse(), Col("x").Eq(p))
		require.NoError(t, err)
		assert.Equal(t, `"x" = ?`, stmt)
		assert.Equal(t, []any{nil}, args)
	})

	t.Run("clickhouse named: nil pointer to Valuer normalizes to untyped nil, not IS NULL", func(t *testing.T) {
		var p *sql.NullString
		stmt, args, err := render(ClickHouseNamed(), Col("x").Eq(p))
		require.NoError(t, err)
		assert.Equal(t, `"x" = @p1`, stmt)
		require.Len(t, args, 1)
		na, ok := args[0].(sql.NamedArg)
		require.True(t, ok)
		assert.Equal(t, "p1", na.Name)
		assert.Nil(t, na.Value)
	})

	// UPDATE is not supported on ClickHouse (FeatureUpdate), so Set is
	// only exercised on Postgres and SQLite; ClickHouse's own binding
	// path is covered by the Eq/Val cases above.
	t.Run("set nil Valuer pointer normalizes to untyped nil", func(t *testing.T) {
		var p *sql.NullString
		for _, d := range []Dialect{Postgres(), SQLite()} {
			_, args, err := Update("t").Set("c", p).Where(Col("id").Eq(1)).Build(d)
			require.NoError(t, err)
			assert.Equal(t, nil, args[0])
		}
	})
}

type valuerSlice []int

func (v valuerSlice) Value() (driver.Value, error) { return "x", nil }

func TestInExpansion(t *testing.T) {
	u := uuid.New()
	tests := []struct {
		name string
		expr Expr
		sql  string
		args []any
	}{
		{"int slice", Col("a").In([]int{1, 2}), `"a" IN ($1, $2)`, []any{1, 2}},
		{"string slice", Col("a").In([]string{"x", "y"}), `"a" IN ($1, $2)`, []any{"x", "y"}},
		{"any slice", Col("a").In([]any{1, "y"}), `"a" IN ($1, $2)`, []any{1, "y"}},
		{"uuid", Col("a").In(u), `"a" IN ($1)`, []any{u}},
		{"bytes", Col("a").In([]byte("ab")), `"a" IN ($1)`, []any{[]byte("ab")}},
		{"json raw", Col("a").In(json.RawMessage(`{}`)), `"a" IN ($1)`, []any{json.RawMessage(`{}`)}},
		{"net ip", Col("a").In(net.ParseIP("127.0.0.1")), `"a" IN ($1)`, []any{net.ParseIP("127.0.0.1")}},
		{"valuer slice", Col("a").In(valuerSlice{1, 2}), `"a" IN ($1)`, []any{valuerSlice{1, 2}}},
		{"int array", Col("a").In([2]int{1, 2}), `"a" IN ($1)`, []any{[2]int{1, 2}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args, err := render(Postgres(), tt.expr)
			require.NoError(t, err)
			assert.Equal(t, tt.sql, sql)
			assert.Equal(t, tt.args, args)
		})
	}
}

func TestBooleanLogic(t *testing.T) {
	sql, args, err := render(Postgres(), Or(And(Col("a").Eq(1), Col("b").Eq(2)), Col("c").Eq(3)))
	require.NoError(t, err)
	assert.Equal(t, `(("a" = $1 AND "b" = $2) OR "c" = $3)`, sql)
	assert.Equal(t, []any{1, 2, 3}, args)

	sql, args, err = render(Postgres(), And(Col("t").Eq(1), Raw("a OR b")))
	require.NoError(t, err)
	assert.Equal(t, `("t" = $1 AND (a OR b))`, sql)
	assert.Equal(t, []any{1}, args)

	sql, args, err = render(Postgres(), Not(Col("a").Eq(1)))
	require.NoError(t, err)
	assert.Equal(t, `NOT ("a" = $1)`, sql)
	assert.Equal(t, []any{1}, args)
}

func TestMatch(t *testing.T) {
	sql, args, err := render(Postgres(), Match(map[string]any{"b": 2, "a": 1}))
	require.NoError(t, err)
	assert.Equal(t, `("a" = $1 AND "b" = $2)`, sql)
	assert.Equal(t, []any{1, 2}, args)

	sql, args, err = render(Postgres(), Match(map[string]any{"a": nil}))
	require.NoError(t, err)
	assert.Equal(t, `"a" IS NULL`, sql)
	assert.Equal(t, []any{}, args)

	_, _, err = render(Postgres(), Match(map[string]any{}))
	assert.True(t, errors.Is(err, ErrEmptyMatch))
}

func TestLikeHelpers(t *testing.T) {
	sql, args, err := render(Postgres(), Col("a").Contains("50%_x!"))
	require.NoError(t, err)
	assert.Equal(t, `"a" LIKE $1 ESCAPE '!'`, sql)
	assert.Equal(t, []any{"%50!%!_x!!%"}, args)

	sql, args, err = render(Postgres(), Col("a").HasPrefix("50%_x!"))
	require.NoError(t, err)
	assert.Equal(t, `"a" LIKE $1 ESCAPE '!'`, sql)
	assert.Equal(t, []any{"50!%!_x!!%"}, args)

	sql, args, err = render(Postgres(), Col("a").HasSuffix("50%_x!"))
	require.NoError(t, err)
	assert.Equal(t, `"a" LIKE $1 ESCAPE '!'`, sql)
	assert.Equal(t, []any{"%50!%!_x!!"}, args)

	sql, args, err = render(ClickHouse(), Col("a").Contains(`50%_x\`))
	require.NoError(t, err)
	assert.Equal(t, `"a" LIKE ?`, sql)
	assert.Equal(t, []any{`%50\%\_x\\%`}, args)
}

func TestFoldLikeHelpers(t *testing.T) {
	named := func(v string) []any { return []any{sql.NamedArg{Name: "p1", Value: v}} }
	tests := []struct {
		name    string
		dialect Dialect
		expr    Expr
		sql     string
		args    []any
	}{
		{"contains pg", Postgres(), Col("a").ContainsFold("AbC%_!"), `"a" ILIKE $1 ESCAPE '!'`, []any{"%AbC!%!_!!%"}},
		{"prefix pg", Postgres(), Col("a").HasPrefixFold("AbC%_!"), `"a" ILIKE $1 ESCAPE '!'`, []any{"AbC!%!_!!%"}},
		{"suffix pg", Postgres(), Col("a").HasSuffixFold("AbC%_!"), `"a" ILIKE $1 ESCAPE '!'`, []any{"%AbC!%!_!!"}},
		{"contains sqlite", SQLite(), Col("a").ContainsFold("AbC%_!"), "`a` LIKE ? ESCAPE '!'", []any{"%AbC!%!_!!%"}},
		{"prefix sqlite", SQLite(), Col("a").HasPrefixFold("AbC%_!"), "`a` LIKE ? ESCAPE '!'", []any{"AbC!%!_!!%"}},
		{"suffix sqlite", SQLite(), Col("a").HasSuffixFold("AbC%_!"), "`a` LIKE ? ESCAPE '!'", []any{"%AbC!%!_!!"}},
		{"contains ch", ClickHouse(), Col("a").ContainsFold(`AbC%_\!`), `"a" ILIKE ?`, []any{`%AbC\%\_\\!%`}},
		{"prefix ch", ClickHouse(), Col("a").HasPrefixFold(`AbC%_\!`), `"a" ILIKE ?`, []any{`AbC\%\_\\!%`}},
		{"suffix ch", ClickHouse(), Col("a").HasSuffixFold(`AbC%_\!`), `"a" ILIKE ?`, []any{`%AbC\%\_\\!`}},
		{"contains ch named", ClickHouseNamed(), Col("a").ContainsFold(`AbC%_\!`), `"a" ILIKE @p1`, named(`%AbC\%\_\\!%`)},
		{"prefix ch named", ClickHouseNamed(), Col("a").HasPrefixFold(`AbC%_\!`), `"a" ILIKE @p1`, named(`AbC\%\_\\!%`)},
		{"suffix ch named", ClickHouseNamed(), Col("a").HasSuffixFold(`AbC%_\!`), `"a" ILIKE @p1`, named(`%AbC\%\_\\!`)},
		{"empty pg", Postgres(), Col("a").ContainsFold(""), `"a" ILIKE $1 ESCAPE '!'`, []any{"%%"}},
		{"escape only pg", Postgres(), Col("a").HasPrefixFold("!!"), `"a" ILIKE $1 ESCAPE '!'`, []any{"!!!!%"}},
		{"backslash pg", Postgres(), Col("a").ContainsFold(`a\b`), `"a" ILIKE $1 ESCAPE '!'`, []any{`%a\b%`}},
		{"bang ch", ClickHouse(), Col("a").ContainsFold("a!b"), `"a" ILIKE ?`, []any{"%a!b%"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args, err := render(tt.dialect, tt.expr)
			require.NoError(t, err)
			assert.Equal(t, tt.sql, sql)
			assert.Equal(t, tt.args, args)
		})
	}

	// The fold helpers escape exactly as the case-sensitive ones do.
	for _, d := range []Dialect{Postgres(), SQLite(), ClickHouse(), ClickHouseNamed()} {
		for _, s := range []string{"AbC%_!", `x\y`, "", "%%__!!"} {
			pairs := [][2]Expr{
				{Col("a").Contains(s), Col("a").ContainsFold(s)},
				{Col("a").HasPrefix(s), Col("a").HasPrefixFold(s)},
				{Col("a").HasSuffix(s), Col("a").HasSuffixFold(s)},
			}
			for _, p := range pairs {
				_, cs, err := render(d, p[0])
				require.NoError(t, err)
				_, ci, err := render(d, p[1])
				require.NoError(t, err)
				assert.Equal(t, cs, ci, "%s %q", d.Name(), s)
			}
		}
	}

	for _, e := range []Expr{Col("a").ContainsFold("x"), Col("a").HasPrefixFold("x"), Col("a").HasSuffixFold("x")} {
		_, _, err := render(Generic(), e)
		assert.True(t, errors.Is(err, ErrUnsupported), "got %v", err)
	}
}

func TestRaw(t *testing.T) {
	arr := []int{1, 2, 3}
	sql, args, err := render(Postgres(), Raw("x @> ?::int[] AND y = '??'", arr))
	require.NoError(t, err)
	assert.Equal(t, `x @> $1::int[] AND y = '?'`, sql)
	assert.Equal(t, []any{arr}, args)

	sql, args, err = render(Postgres(), Raw("??"))
	require.NoError(t, err)
	assert.Equal(t, "?", sql)
	assert.Equal(t, []any{}, args)

	_, _, err = render(ClickHouse(), Raw("??"))
	assert.True(t, errors.Is(err, ErrRawPlaceholder))

	sql, args, err = render(Postgres(), Raw("lower(?)", Col("a")))
	require.NoError(t, err)
	assert.Equal(t, `lower("a")`, sql)
	assert.Equal(t, []any{}, args)

	_, _, err = render(Postgres(), Raw("? ?", 1))
	assert.True(t, errors.Is(err, ErrRawArgs))

	forbidden := []string{"a $1 b", "a ?1 b", "a -? b", "a $? b", `a \? b`}
	for _, dialect := range []Dialect{Postgres(), SQLite(), ClickHouse(), Generic()} {
		for _, s := range forbidden {
			t.Run(dialect.Name()+"/"+s, func(t *testing.T) {
				_, _, err := render(dialect, Raw(s))
				assert.True(t, errors.Is(err, ErrRawPlaceholder), "got %v", err)
			})
		}
	}
}

func TestInt(t *testing.T) {
	sql, _, err := render(Postgres(), Int(7))
	require.NoError(t, err)
	assert.Equal(t, "7", sql)

	sql, _, err = render(Postgres(), Int(-3))
	require.NoError(t, err)
	assert.Equal(t, "(-3)", sql)

	_, _, err = render(Postgres(), Raw("x -?", Int(-3)))
	assert.True(t, errors.Is(err, ErrRawPlaceholder))
}

func TestNilExpr(t *testing.T) {
	_, _, err := render(Postgres(), And(nil))
	assert.True(t, errors.Is(err, ErrNilExpr))

	_, _, err = render(Postgres(), Not(nil))
	assert.True(t, errors.Is(err, ErrNilExpr))

	_, _, err = render(Postgres(), Col("a").Eq(Value{}))
	assert.True(t, errors.Is(err, ErrNilExpr))
}

func TestTrivialFlag(t *testing.T) {
	assert.True(t, And().trivial)
	assert.True(t, And(And()).trivial)
	assert.True(t, Col("a").NotIn().trivial)

	assert.False(t, And(Col("a").Eq(1)).trivial)
	assert.False(t, Or().trivial)
	assert.False(t, Col("a").In().trivial)

	// Not swaps trivial/never.
	assert.True(t, Not(Col("a").In()).trivial)
	assert.False(t, Not(Col("a").NotIn()).trivial)
	assert.True(t, Not(Col("a").NotIn()).never)

	// Or is trivial if any child is trivial.
	assert.True(t, Or(Col("a").Eq(1), Col("b").NotIn()).trivial)

	// Or() is never.
	assert.True(t, Or().never)

	// Val copies both flags.
	assert.True(t, Val(And()).trivial)

	// Not(Raw(...)) is neither (Raw carries no flags).
	assert.False(t, Not(Raw("x")).trivial)
	assert.False(t, Not(Raw("x")).never)

	// And is never if any child is never.
	assert.True(t, Not(And(Col("a").Eq(1), Col("b").In())).trivial)

	// Or is never only if all children are never.
	assert.True(t, Not(Or(Col("a").In(), Col("b").In())).trivial)

	// Subquery propagation: never WHERE inside NotIn/NotExists -> trivial.
	assert.True(t, Col("id").NotIn(Select("id").From("x").Where(Col("g").In())).trivial)
	assert.True(t, NotExists(Select(Int(1)).From("x").Where(Col("k").In())).trivial)
}

func TestValPreservesRawParens(t *testing.T) {
	sql, _, err := render(Postgres(), Val(Raw("a OR b")).Eq(1))
	require.NoError(t, err)
	assert.Equal(t, `(a OR b) = $1`, sql)
}

func TestAliasSinglePart(t *testing.T) {
	_, _, err := render(Postgres(), Col("a").As("x.y"))
	assert.True(t, errors.Is(err, ErrInvalidIdentifier))

	_, _, err = render(Postgres(), Col("a").As("*"))
	assert.True(t, errors.Is(err, ErrInvalidIdentifier))
}

func TestOrder(t *testing.T) {
	sql, _, err := render(Postgres(), Col("a").Asc())
	require.NoError(t, err)
	assert.Equal(t, `"a" ASC`, sql)

	sql, _, err = render(Postgres(), Col("a").Desc())
	require.NoError(t, err)
	assert.Equal(t, `"a" DESC`, sql)

	sql, _, err = render(Postgres(), Col("a").Asc().NullsFirst())
	require.NoError(t, err)
	assert.Equal(t, `"a" ASC NULLS FIRST`, sql)

	sql, _, err = render(Postgres(), Col("a").Desc().NullsLast())
	require.NoError(t, err)
	assert.Equal(t, `"a" DESC NULLS LAST`, sql)
}

func TestInSubquery(t *testing.T) {
	inner := Select("uid").From("x").Where(Col("k").Eq(1))
	sql, args, err := render(Postgres(), Col("id").In(inner))
	require.NoError(t, err)
	assert.Equal(t, `"id" IN (SELECT "uid" FROM "x" WHERE "k" = $1)`, sql)
	assert.Equal(t, []any{1}, args)

	sql, args, err = render(Postgres(), Col("id").NotIn(inner))
	require.NoError(t, err)
	assert.Equal(t, `"id" NOT IN (SELECT "uid" FROM "x" WHERE "k" = $1)`, sql)
	assert.Equal(t, []any{1}, args)

	sql, args, err = render(ClickHouse(), Col("id").In(Select("uid").From("x")))
	require.NoError(t, err)
	assert.Equal(t, `"id" IN (SELECT "uid" FROM "x")`, sql)
	assert.Equal(t, []any{}, args)

	_, _, err = render(Postgres(), Col("id").In((*SelectBuilder)(nil)))
	assert.True(t, errors.Is(err, ErrNilExpr))
}

func TestExists(t *testing.T) {
	sql, args, err := render(Postgres(), Exists(Select(Int(1)).From("u")))
	require.NoError(t, err)
	assert.Equal(t, `EXISTS (SELECT 1 FROM "u")`, sql)
	assert.Equal(t, []any{}, args)

	sql, args, err = render(Postgres(), NotExists(Select(Int(1)).From("u")))
	require.NoError(t, err)
	assert.Equal(t, `NOT EXISTS (SELECT 1 FROM "u")`, sql)
	assert.Equal(t, []any{}, args)

	sql, args, err = render(ClickHouse(), Exists(Select(Int(1)).From("u")))
	require.NoError(t, err)
	assert.Equal(t, `EXISTS (SELECT 1 FROM "u")`, sql)
	assert.Equal(t, []any{}, args)

	_, _, err = render(Postgres(), Exists(nil))
	assert.True(t, errors.Is(err, ErrNilExpr))
}

func TestSub(t *testing.T) {
	sql, args, err := render(Postgres(), Col("a").Eq(Sub(Select(Max("a")).From("t"))))
	require.NoError(t, err)
	assert.Equal(t, `"a" = (SELECT MAX("a") FROM "t")`, sql)
	assert.Equal(t, []any{}, args)

	sql, args, err = render(ClickHouse(), Col("a").Eq(Sub(Select(Max("a")).From("t"))))
	require.NoError(t, err)
	assert.Equal(t, `"a" = (SELECT MAX("a") FROM "t")`, sql)
	assert.Equal(t, []any{}, args)

	_, _, err = render(Postgres(), Sub(nil))
	assert.True(t, errors.Is(err, ErrNilExpr))
}

func TestValuesNeverInlined(t *testing.T) {
	hostile := `x\' OR 1=1 --`
	for _, dialect := range []Dialect{Postgres(), SQLite(), ClickHouse(), Generic()} {
		t.Run(dialect.Name(), func(t *testing.T) {
			sql, args, err := render(dialect, Col("a").Eq(hostile))
			require.NoError(t, err)
			assert.NotContains(t, sql, "OR 1=1")
			assert.Equal(t, []any{hostile}, args)
		})
	}
}
