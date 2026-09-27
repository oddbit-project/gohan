package gohan

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLockGolden(t *testing.T) {
	tests := []struct {
		name  string
		build *SelectBuilder
		sql   string
		args  []any
	}{
		{
			"for update skip locked",
			From("jobs").Where(Col("state").Eq("new")).OrderBy("id").Limit(1).ForUpdate().SkipLocked(),
			`SELECT * FROM "jobs" WHERE "state" = $1 ORDER BY "id" ASC LIMIT 1 FOR UPDATE SKIP LOCKED`,
			[]any{"new"},
		},
		{
			"for no key update",
			From("jobs").Where(Col("state").Eq("new")).OrderBy("id").Limit(1).ForNoKeyUpdate().SkipLocked(),
			`SELECT * FROM "jobs" WHERE "state" = $1 ORDER BY "id" ASC LIMIT 1 FOR NO KEY UPDATE SKIP LOCKED`,
			[]any{"new"},
		},
		{
			"for share",
			From("jobs").Where(Col("state").Eq("new")).OrderBy("id").Limit(1).ForShare().SkipLocked(),
			`SELECT * FROM "jobs" WHERE "state" = $1 ORDER BY "id" ASC LIMIT 1 FOR SHARE SKIP LOCKED`,
			[]any{"new"},
		},
		{
			"for key share",
			From("jobs").Where(Col("state").Eq("new")).OrderBy("id").Limit(1).ForKeyShare().SkipLocked(),
			`SELECT * FROM "jobs" WHERE "state" = $1 ORDER BY "id" ASC LIMIT 1 FOR KEY SHARE SKIP LOCKED`,
			[]any{"new"},
		},
		{
			"for update nowait",
			From("jobs").Where(Col("state").Eq("new")).OrderBy("id").Limit(1).ForUpdate().NoWait(),
			`SELECT * FROM "jobs" WHERE "state" = $1 ORDER BY "id" ASC LIMIT 1 FOR UPDATE NOWAIT`,
			[]any{"new"},
		},
		{
			"of multiple clauses",
			From(Table("t").As("x")).
				Join(Table("u").As("u"), Table("u").Col("t_id").Eq(Table("x").Col("id"))).
				ForUpdate("x").ForShare("u"),
			`SELECT * FROM "t" AS "x" INNER JOIN "u" AS "u" ON "u"."t_id" = "x"."id" FOR UPDATE OF "x" FOR SHARE OF "u"`,
			[]any{},
		},
		{
			"limit offset lock",
			From("t").Limit(2).Offset(3).ForUpdate(),
			`SELECT * FROM "t" LIMIT 2 OFFSET 3 FOR UPDATE`,
			[]any{},
		},
		{
			"lock inside cte",
			func() *SelectBuilder {
				locked := From("t").Where(Col("id").Eq(1)).ForUpdate()
				return Select("id").With("c", locked).From("c")
			}(),
			`WITH "c" AS (SELECT * FROM "t" WHERE "id" = $1 FOR UPDATE) SELECT "id" FROM "c"`,
			[]any{1},
		},
		{
			"lock inside in subquery",
			func() *SelectBuilder {
				locked := Select("id").From("t").Where(Col("state").Eq("new")).ForUpdate()
				return From("u").Where(Col("id").In(locked))
			}(),
			`SELECT * FROM "u" WHERE "id" IN (SELECT "id" FROM "t" WHERE "state" = $1 FOR UPDATE)`,
			[]any{"new"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args, err := tt.build.Build(Postgres())
			require.NoError(t, err)
			assert.Equal(t, tt.sql, sql)
			assert.Equal(t, tt.args, args)
		})
	}
}

func TestLockErrors(t *testing.T) {
	tests := []struct {
		name  string
		build *SelectBuilder
		err   error
	}{
		{"sqlite unsupported", From("t").ForUpdate(), ErrUnsupported},
		{"clickhouse unsupported", From("t").ForUpdate(), ErrUnsupported},
		{"generic unsupported", From("t").ForUpdate(), ErrUnsupported},
		{"distinct", From("t").Distinct().ForUpdate(), ErrInvalidLock},
		{"group by", From("t").GroupBy("a").ForUpdate(), ErrInvalidLock},
		{"having", From("t").GroupBy("a").Having(CountAll().Gt(1)).ForUpdate(), ErrInvalidLock},
		{"union", From("a").Union(From("b")).ForUpdate(), ErrInvalidLock},
		{"skip locked no lock", From("t").SkipLocked(), ErrInvalidLock},
		{"nowait no lock", From("t").NoWait(), ErrInvalidLock},
		{"skip locked then nowait", From("t").ForUpdate().SkipLocked().NoWait(), ErrInvalidLock},
		{"nowait then skip locked", From("t").ForUpdate().NoWait().SkipLocked(), ErrInvalidLock},
		{"of dotted", From("t").ForUpdate("a.b"), ErrInvalidIdentifier},
		{"of star", From("t").ForUpdate("*"), ErrInvalidIdentifier},
		{"compound member lock", From("a").Union(From("b").ForUpdate()), ErrCompoundPart},
	}
	dialects := map[string]Dialect{
		"sqlite unsupported":     SQLite(),
		"clickhouse unsupported": ClickHouse(),
		"generic unsupported":    Generic(),
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := Postgres()
			if dd, ok := dialects[tt.name]; ok {
				d = dd
			}
			_, _, err := tt.build.Build(d)
			assert.True(t, errors.Is(err, tt.err), "got %v", err)
		})
	}
}

// TestLockClickHouseNamedUnsupported checks that ClickHouseNamed behaves
// like ClickHouse: it does not have FeatureLocking either.
func TestLockClickHouseNamedUnsupported(t *testing.T) {
	_, _, err := From("t").ForUpdate().Build(ClickHouseNamed())
	assert.True(t, errors.Is(err, ErrUnsupported), "got %v", err)
}

func TestLockImmutability(t *testing.T) {
	base := From("t").ForUpdate()
	_ = base.SkipLocked()
	_ = base.NoWait()

	baseSQL, _, err := base.Build(Postgres())
	require.NoError(t, err)
	assert.Equal(t, `SELECT * FROM "t" FOR UPDATE`, baseSQL)

	x := base.ForShare("u")
	y := base.ForKeyShare("v")

	xSQL, _, err := x.Build(Postgres())
	require.NoError(t, err)
	assert.Equal(t, `SELECT * FROM "t" FOR UPDATE FOR SHARE OF "u"`, xSQL)

	ySQL, _, err := y.Build(Postgres())
	require.NoError(t, err)
	assert.Equal(t, `SELECT * FROM "t" FOR UPDATE FOR KEY SHARE OF "v"`, ySQL)

	assert.NotContains(t, xSQL, "KEY SHARE")
}
