package gohan

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsTrivial(t *testing.T) {
	trueCases := []struct {
		name string
		cond Expr
	}{
		{"nil", nil},
		{"zero value", Value{}},
		{"and empty", And()},
		{"and nested empty", And(And())},
		{"or with trivial member", Or(And(), Col("a").Eq(1))},
		{"not or empty", Not(Or())},
		{"raw 1=1", Raw("1=1")},
		{"raw 1 = 1 spaced", Raw("1 = 1")},
		{"raw TRUE", Raw("TRUE")},
		{"raw quoted literals", Raw("'a' = 'a'")},
		{"raw placeholders", Raw("? = ?", 1, 1)},
		{"val eq", Val(1).Eq(1)},
		{"raw not false", Raw("NOT FALSE")},
		{"raw 1=0 constant false is still column-free", Raw("1=0")},
	}
	for _, tc := range trueCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.True(t, IsTrivial(tc.cond), "expected trivial")
		})
	}

	falseCases := []struct {
		name string
		cond Expr
	}{
		{"col eq", Col("a").Eq(1)},
		{"raw identifier", Raw("a = 1")},
		{"raw quoted identifier", Raw("\"a\" > 0")},
		{"raw expr arg is a column", Raw("? @> ?", Col("tags"), []string{"x"})},
		{"raw escaped literal then identifier", Raw("'it''s' = name")},
		{"match", Match(map[string]any{"a": 1})},
		{"exists references table", Exists(Select(Int(1)).From("u"))},
		{"and with one real filter", And(Col("a").Eq(1), Raw("1=1"))},
		{"raw fails validation", Raw("$1 = 1")},
	}
	for _, tc := range falseCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.False(t, IsTrivial(tc.cond), "expected not trivial")
		})
	}
}

func TestIsEmpty(t *testing.T) {
	trueCases := []struct {
		name string
		cond Expr
	}{
		{"nil", nil},
		{"zero value", Value{}},
		{"and empty", And()},
		{"and nested empty", And(And())},
	}
	for _, tc := range trueCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.True(t, IsEmpty(tc.cond), "expected empty")
		})
	}

	falseCases := []struct {
		name string
		cond Expr
	}{
		{"raw 1=1", Raw("1=1")},
		{"or empty", Or()},
	}
	for _, tc := range falseCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.False(t, IsEmpty(tc.cond), "expected not empty")
		})
	}
}

func TestTrivialGuard(t *testing.T) {
	_, _, err := Delete("t").Where(Raw("1=1")).Build(Postgres())
	assert.True(t, errors.Is(err, ErrNoWhere))

	_, _, err = Update("t").Set("a", 1).Where(Val(1).Eq(1)).Build(Postgres())
	assert.True(t, errors.Is(err, ErrNoWhere))

	sql, _, err := Delete("t").Where(Raw("1=1")).All().Build(Postgres())
	require.NoError(t, err)
	assert.Equal(t, `DELETE FROM "t" WHERE (1=1)`, sql)

	sql, _, err = Update("t").Set("a", 1).Where(Val(1).Eq(1)).All().Build(Postgres())
	require.NoError(t, err)
	assert.Equal(t, `UPDATE "t" SET "a" = $1 WHERE $2 = $3`, sql)

	// A real filter still builds without All().
	sql, _, err = Delete("t").Where(Raw("a = 1")).Build(Postgres())
	require.NoError(t, err)
	assert.Equal(t, `DELETE FROM "t" WHERE (a = 1)`, sql)
}

func TestIsTrivialPurity(t *testing.T) {
	cond := Col("a").Eq(1)

	before, argsBefore, errBefore := render(Postgres(), cond)
	require.NoError(t, errBefore)

	assert.False(t, IsTrivial(cond))
	assert.False(t, IsTrivial(cond))

	after, argsAfter, errAfter := render(Postgres(), cond)
	require.NoError(t, errAfter)

	assert.Equal(t, before, after)
	assert.Equal(t, argsBefore, argsAfter)
}
