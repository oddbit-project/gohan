package gohan

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runNoPanic runs fn, failing the test with the recovered panic value
// instead of letting it propagate, so a regression to the pre-fix
// nil-pointer panic is reported as a normal test failure.
func runNoPanic(t *testing.T, name string, fn func() error) {
	t.Run(name, func(t *testing.T) {
		var err error
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic: %v", r)
				}
			}()
			err = fn()
		}()
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrNilExpr), "got %v", err)
	})
}

// TestNilExprPointer checks that a nil *Value/*Order, reached through
// every kind of entry point that accepts an Expr, fails with ErrNilExpr
// instead of panicking (the typed-nil-pointer hazard: *Value and *Order
// satisfy Expr through a value-receiver render method, so e == nil does
// not catch them).
func TestNilExprPointer(t *testing.T) {
	var nv *Value
	var no *Order
	a1 := Col("a").Eq(1)

	runNoPanic(t, "Where", func() error {
		_, _, err := From("t").Where(nv).Build(Postgres())
		return err
	})
	runNoPanic(t, "Where order", func() error {
		_, _, err := From("t").Where(no).Build(Postgres())
		return err
	})
	runNoPanic(t, "And", func() error {
		_, _, err := render(Postgres(), And(nv, a1))
		return err
	})
	runNoPanic(t, "Or", func() error {
		_, _, err := render(Postgres(), Or(nv))
		return err
	})
	runNoPanic(t, "Not", func() error {
		_, _, err := render(Postgres(), Not(nv))
		return err
	})
	runNoPanic(t, "Eq", func() error {
		_, _, err := render(Postgres(), Col("a").Eq(nv))
		return err
	})
	runNoPanic(t, "Neq", func() error {
		_, _, err := render(Postgres(), Col("a").Neq(nv))
		return err
	})
	runNoPanic(t, "Gt", func() error {
		_, _, err := render(Postgres(), Col("a").Gt(nv))
		return err
	})
	runNoPanic(t, "In", func() error {
		_, _, err := render(Postgres(), Col("a").In(1, nv))
		return err
	})
	runNoPanic(t, "Between", func() error {
		_, _, err := render(Postgres(), Col("a").Between(nv, 1))
		return err
	})
	runNoPanic(t, "Like", func() error {
		_, _, err := render(Postgres(), Col("a").Like(nv))
		return err
	})
	runNoPanic(t, "ILike", func() error {
		_, _, err := render(Postgres(), Col("a").ILike(nv))
		return err
	})
	runNoPanic(t, "Val", func() error {
		_, _, err := render(Postgres(), Val(nv))
		return err
	})
	runNoPanic(t, "Raw", func() error {
		_, _, err := render(Postgres(), Raw("a = ?", nv))
		return err
	})
	runNoPanic(t, "Match", func() error {
		_, _, err := render(Postgres(), Match(map[string]any{"a": nv}))
		return err
	})
	runNoPanic(t, "Fn", func() error {
		_, _, err := render(Postgres(), Fn("f", nv))
		return err
	})
	runNoPanic(t, "Cast", func() error {
		_, _, err := render(Postgres(), Cast(nv, "int"))
		return err
	})
	runNoPanic(t, "Count", func() error {
		_, _, err := render(Postgres(), Count(nv))
		return err
	})
	runNoPanic(t, "Case When cond", func() error {
		_, _, err := render(Postgres(), Case().When(nv, 1).End())
		return err
	})
	runNoPanic(t, "Case When then", func() error {
		_, _, err := render(Postgres(), Case().When(a1, nv).End())
		return err
	})
	runNoPanic(t, "Case Else", func() error {
		_, _, err := render(Postgres(), Case().When(a1, 1).Else(nv))
		return err
	})
	runNoPanic(t, "Select", func() error {
		_, _, err := Select(nv).From("t").Build(Postgres())
		return err
	})
	runNoPanic(t, "Select order", func() error {
		_, _, err := Select(no).From("t").Build(Postgres())
		return err
	})
	runNoPanic(t, "GroupBy", func() error {
		_, _, err := From("t").GroupBy(nv).Build(Postgres())
		return err
	})
	runNoPanic(t, "Having", func() error {
		_, _, err := From("t").Having(nv).Build(Postgres())
		return err
	})
	runNoPanic(t, "OrderBy", func() error {
		_, _, err := From("t").OrderBy(no).Build(Postgres())
		return err
	})
	runNoPanic(t, "OrderBy value", func() error {
		_, _, err := From("t").OrderBy(nv).Build(Postgres())
		return err
	})
	runNoPanic(t, "Update Returning", func() error {
		_, _, err := Update("t").Set("a", 1).All().Returning(nv).Build(Postgres())
		return err
	})
	runNoPanic(t, "Insert Returning", func() error {
		_, _, err := Insert("t").Columns("a").Values(1).Returning(nv).Build(Postgres())
		return err
	})
	runNoPanic(t, "Join ON", func() error {
		_, _, err := From("t").Join("u", no).Build(Postgres())
		return err
	})
	runNoPanic(t, "Join ON value", func() error {
		_, _, err := From("t").Join("u", nv).Build(Postgres())
		return err
	})
	runNoPanic(t, "Update Set", func() error {
		_, _, err := Update("t").Set("a", nv).All().Build(Postgres())
		return err
	})
	runNoPanic(t, "Update SetMap", func() error {
		_, _, err := Update("t").SetMap(map[string]any{"a": nv}).All().Build(Postgres())
		return err
	})
	runNoPanic(t, "Insert Values", func() error {
		_, _, err := Insert("t").Columns("a").Values(nv).Build(Postgres())
		return err
	})
	runNoPanic(t, "Insert SetMap", func() error {
		_, _, err := Insert("t").SetMap(map[string]any{"a": nv}).Build(Postgres())
		return err
	})
	runNoPanic(t, "OnConflict DoUpdate", func() error {
		_, _, err := Insert("t").Columns("a").Values(1).OnConflict("id").
			DoUpdate(map[string]any{"a": nv}).Build(Postgres())
		return err
	})
	runNoPanic(t, "Prewhere", func() error {
		_, _, err := From("t").Prewhere(nv).Build(ClickHouse())
		return err
	})
	runNoPanic(t, "ArrayJoin", func() error {
		_, _, err := Select("x").From("t").ArrayJoin(nv).Build(ClickHouse())
		return err
	})
	runNoPanic(t, "Settings", func() error {
		_, _, err := From("t").Settings(map[string]any{"x": nv}).Build(ClickHouse())
		return err
	})
	runNoPanic(t, "Exists", func() error {
		_, _, err := render(Postgres(), Exists(From("t").Where(nv)))
		return err
	})

	t.Run("IsEmpty", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("panic: %v", r)
			}
		}()
		assert.True(t, IsEmpty(nv))
		assert.True(t, IsEmpty(no))
	})
	t.Run("IsTrivial", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("panic: %v", r)
			}
		}()
		assert.True(t, IsTrivial(Expr(nv)))
		assert.True(t, IsTrivial(Expr(no)))
	})
	t.Run("IsTrivial And nv", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("panic: %v", r)
			}
		}()
		// Unlike a bare nil *Value, wrapping it in And() makes IsTrivial
		// render and observe the ErrNilExpr failure, so it is false.
		assert.False(t, IsTrivial(And(nv)))
	})
}

// TestNonNilPointerParity checks the HIGH-severity companion bug: today,
// every flag reader type-asserts e.(Value), which never matches a
// *Value/*Order pointer, so a non-nil *Value silently lost its
// isRaw/trivial/never/empty classification. normExpr dereferences the
// pointer first, so a non-nil *Value/*Order now renders and classifies
// exactly like the value it points to.
func TestNonNilPointerParity(t *testing.T) {
	a1 := Col("a").Eq(1)

	t.Run("IsTrivial And", func(t *testing.T) {
		v := And()
		assert.Equal(t, IsTrivial(v), IsTrivial(&v))
	})
	t.Run("IsTrivial Or", func(t *testing.T) {
		v := Or()
		assert.Equal(t, IsTrivial(v), IsTrivial(&v))
	})
	t.Run("IsTrivial Raw", func(t *testing.T) {
		v := Raw("1=1")
		assert.Equal(t, IsTrivial(v), IsTrivial(&v))
	})
	t.Run("IsTrivial Eq", func(t *testing.T) {
		v := a1
		assert.Equal(t, IsTrivial(v), IsTrivial(&v))
	})
	t.Run("IsEmpty And", func(t *testing.T) {
		v := And()
		assert.Equal(t, IsEmpty(v), IsEmpty(&v))
	})
	t.Run("IsEmpty Or", func(t *testing.T) {
		v := Or()
		assert.Equal(t, IsEmpty(v), IsEmpty(&v))
	})
	t.Run("IsEmpty Raw", func(t *testing.T) {
		v := Raw("1=1")
		assert.Equal(t, IsEmpty(v), IsEmpty(&v))
	})
	t.Run("IsEmpty Eq", func(t *testing.T) {
		v := a1
		assert.Equal(t, IsEmpty(v), IsEmpty(&v))
	})
	t.Run("Delete Where trivially-true Or", func(t *testing.T) {
		orVal := Or(a1, And())
		_, _, err := Delete("t").Where(&orVal).Build(Postgres())
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrNoWhere), "got %v", err)

		// Parity: the value form fails identically.
		_, _, errValue := Delete("t").Where(orVal).Build(Postgres())
		require.Error(t, errValue)
		assert.True(t, errors.Is(errValue, ErrNoWhere), "got %v", errValue)
	})
	t.Run("Where Raw parenthesized", func(t *testing.T) {
		raw := Raw("a = 1 OR b = 2")
		sqlPtr, argsPtr, errPtr := From("t").Where(&raw, a1).Build(Postgres())
		require.NoError(t, errPtr)
		sqlVal, argsVal, errVal := From("t").Where(raw, a1).Build(Postgres())
		require.NoError(t, errVal)
		assert.Equal(t, sqlVal, sqlPtr)
		assert.Equal(t, argsVal, argsPtr)
		assert.Contains(t, sqlPtr, `(a = 1 OR b = 2)`)
	})
	t.Run("OrderBy pointer", func(t *testing.T) {
		order := Col("a").Desc()
		sqlPtr, _, errPtr := From("t").OrderBy(&order).Build(Postgres())
		require.NoError(t, errPtr)
		sqlVal, _, errVal := From("t").OrderBy(order).Build(Postgres())
		require.NoError(t, errVal)
		assert.Equal(t, sqlVal, sqlPtr)
	})
}
