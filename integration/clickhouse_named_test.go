// This file exercises gohan.ClickHouseNamed against a real ClickHouse
// server: it proves the sub-second precision loss of the default
// gohan.ClickHouse() positional mode, that clickhouse.DateNamed recovers
// full precision through the named mode, exact string round trips, and
// records the native clickhouse.Conn API's behaviour with sql.NamedArg
// (Step 5 of plans/012-clickhouse-named-parameters.md).
package integration

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/oddbit-project/gohan"
	"github.com/stretchr/testify/require"
)

// chDSN returns the ClickHouse DSN for these tests, or skips the test if
// it is not set (mirrors engines()'s GOHAN_CH_DSN handling, but this file
// needs a ClickHouse-only *sql.DB and a native clickhouse.Conn, which the
// shared engine harness does not provide).
func chDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("GOHAN_CH_DSN")
	if dsn == "" {
		t.Skip("GOHAN_CH_DSN not set")
	}
	return dsn
}

func chStdDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("clickhouse", chDSN(t))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	return db
}

// namedArgs converts a rendered gohan.ClickHouseNamed() arg slice for use
// with database/sql: each sql.NamedArg whose Value is a time.Time is
// replaced with clickhouse.DateNamed(name, t, scale) (per the documented
// recipe); every other arg is left as the sql.NamedArg gohan produced.
func namedArgs(t *testing.T, args []any, scale clickhouse.TimeUnit) []any {
	t.Helper()
	out := make([]any, len(args))
	for i, a := range args {
		na, ok := a.(sql.NamedArg)
		require.True(t, ok, "arg %d is not sql.NamedArg: %T", i, a)
		if tm, ok := na.Value.(time.Time); ok {
			out[i] = clickhouse.DateNamed(na.Name, tm, scale)
			continue
		}
		out[i] = na
	}
	return out
}

// chTable returns a fresh table name for db and registers a cleanup that
// drops it. Mirrors newTable/tableSeq in conformance_test.go, which take
// an engine rather than a bare *sql.DB.
func chTable(t *testing.T, db *sql.DB) string {
	t.Helper()
	name := fmt.Sprintf("t_%d", tableSeq.Add(1))
	t.Cleanup(func() { _, _ = db.Exec("DROP TABLE IF EXISTS " + name) })
	return name
}

func chToString(t *testing.T, db *sql.DB, expr, table string, id int) string {
	t.Helper()
	var s string
	err := db.QueryRow(fmt.Sprintf("SELECT toString(%s) FROM %s WHERE id = %d", expr, table, id)).Scan(&s)
	require.NoError(t, err)
	return s
}

// TestClickHouseNamedPrecision covers Step 5, points 1-2 of plan 012: the
// default ClickHouse() dialect loses sub-second precision through
// clickhouse-go's positional binding, and ClickHouseNamed() with
// clickhouse.DateNamed recovers it.
func TestClickHouseNamedPrecision(t *testing.T) {
	db := chStdDB(t)
	tbl := chTable(t, db)
	_, err := db.Exec(fmt.Sprintf(
		"CREATE TABLE %s (id UInt32, t3 DateTime64(3), t6 DateTime64(6), t9 DateTime64(9)) ENGINE = MergeTree ORDER BY id",
		tbl))
	require.NoError(t, err)

	tm := time.Date(2026, 9, 27, 12, 34, 56, 123456789, time.UTC)

	// 1. Positional ClickHouse() insert: documented precision loss.
	q, args, err := gohan.Insert(tbl).Columns("id", "t3", "t6", "t9").Values(1, tm, tm, tm).Build(gohan.ClickHouse())
	require.NoError(t, err)
	_, err = db.Exec(q, args...)
	require.NoError(t, err)

	got9 := chToString(t, db, "t9", tbl, 1)
	require.Equal(t, "2026-09-27 12:34:56.000000000", got9,
		"positional ClickHouse() insert must lose sub-second precision (documented loss)")

	// 2. ClickHouseNamed() insert with time args converted to
	// clickhouse.DateNamed(..., NanoSeconds): full precision.
	q, args, err = gohan.Insert(tbl).Columns("id", "t3", "t6", "t9").Values(2, tm, tm, tm).Build(gohan.ClickHouseNamed())
	require.NoError(t, err)
	_, err = db.Exec(q, namedArgs(t, args, clickhouse.NanoSeconds)...)
	require.NoError(t, err)

	require.Equal(t, "2026-09-27 12:34:56.123", chToString(t, db, "t3", tbl, 2))
	require.Equal(t, "2026-09-27 12:34:56.123456", chToString(t, db, "t6", tbl, 2))
	require.Equal(t, "2026-09-27 12:34:56.123456789", chToString(t, db, "t9", tbl, 2))

	// A WHERE t9 = @p1 select with the converted arg finds the row.
	q, args, err = gohan.Select("id").From(tbl).Where(gohan.Col("t9").Eq(tm)).Build(gohan.ClickHouseNamed())
	require.NoError(t, err)
	var id int
	err = db.QueryRow(q, namedArgs(t, args, clickhouse.NanoSeconds)...).Scan(&id)
	require.NoError(t, err)
	require.Equal(t, 2, id)
}

// TestClickHouseNamedDateAndTimezone covers Step 5, point 2b: Date,
// Date32 and a non-UTC DateTime64 column compare correctly against a
// DateNamed argument, and exact-instant semantics for DateTime64(3).
func TestClickHouseNamedDateAndTimezone(t *testing.T) {
	db := chStdDB(t)
	tbl := chTable(t, db)
	_, err := db.Exec(fmt.Sprintf(
		"CREATE TABLE %s (id UInt32, t3 DateTime64(3)) ENGINE = MergeTree ORDER BY id", tbl))
	require.NoError(t, err)
	_, err = db.Exec(fmt.Sprintf(
		"ALTER TABLE %s ADD COLUMN d Date, ADD COLUMN d32 Date32, ADD COLUMN tl DateTime64(6, 'Europe/Lisbon')", tbl))
	require.NoError(t, err)

	tm := time.Date(2026, 9, 27, 12, 34, 56, 123456789, time.UTC)
	tmMilli := tm.Truncate(time.Millisecond)
	tmMicro := tm.Truncate(time.Microsecond)
	tmMidnight := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)

	// Row 2: t3 stored at millisecond precision (.123), per the
	// TestClickHouseNamedPrecision case above but self-contained here.
	q, args, err := gohan.Insert(tbl).Columns("id", "t3").Values(2, tm).Build(gohan.ClickHouseNamed())
	require.NoError(t, err)
	_, err = db.Exec(q, namedArgs(t, args, clickhouse.NanoSeconds)...)
	require.NoError(t, err)
	require.Equal(t, "2026-09-27 12:34:56.123", chToString(t, db, "t3", tbl, 2))

	// Row 3: d, d32, tl.
	q, args, err = gohan.Insert(tbl).Columns("id", "d", "d32", "tl").
		Values(3, tmMidnight, tmMidnight, tm).
		Build(gohan.ClickHouseNamed())
	require.NoError(t, err)
	_, err = db.Exec(q, namedArgs(t, args, clickhouse.NanoSeconds)...)
	require.NoError(t, err)
	require.Equal(t, "2026-09-27", chToString(t, db, "d", tbl, 3))
	require.Equal(t, "2026-09-27", chToString(t, db, "d32", tbl, 3))
	// tl is DateTime64(6, 'Europe/Lisbon'): stored value keeps the same
	// instant, truncated to microseconds; toString renders it in its own
	// column timezone, so compare via toUnixTimestamp64Micro instead.
	var tlMicro int64
	require.NoError(t, db.QueryRow(
		fmt.Sprintf("SELECT toUnixTimestamp64Micro(tl) FROM %s WHERE id = 3", tbl)).Scan(&tlMicro))
	require.Equal(t, tmMicro.UnixMicro(), tlMicro)

	selectID := func(col string, tm time.Time) (int, error) {
		q, args, err := gohan.Select("id").From(tbl).Where(gohan.Col(col).Eq(tm)).Build(gohan.ClickHouseNamed())
		require.NoError(t, err)
		var id int
		err = db.QueryRow(q, namedArgs(t, args, clickhouse.NanoSeconds)...).Scan(&id)
		return id, err
	}

	t.Run("d matches a UTC-midnight time", func(t *testing.T) {
		id, err := selectID("d", tmMidnight)
		require.NoError(t, err)
		require.Equal(t, 3, id)
	})

	t.Run("d32 matches a UTC-midnight time", func(t *testing.T) {
		id, err := selectID("d32", tmMidnight)
		require.NoError(t, err)
		require.Equal(t, 3, id)
	})

	t.Run("tl matches tm truncated to microseconds", func(t *testing.T) {
		id, err := selectID("tl", tmMicro)
		require.NoError(t, err)
		require.Equal(t, 3, id)
	})

	t.Run("t3 does not match the full-nanosecond instant", func(t *testing.T) {
		_, err := selectID("t3", tm)
		require.ErrorIs(t, err, sql.ErrNoRows)
	})

	t.Run("t3 matches when tm is truncated to milliseconds", func(t *testing.T) {
		id, err := selectID("t3", tmMilli)
		require.NoError(t, err)
		require.Equal(t, 2, id)
	})
}

// TestClickHouseNamedStringRoundTrip covers Step 5, point 3: exact string
// round trips through named-mode binding.
func TestClickHouseNamedStringRoundTrip(t *testing.T) {
	db := chStdDB(t)
	tbl := chTable(t, db)
	_, err := db.Exec(fmt.Sprintf(
		"CREATE TABLE %s (id UInt32, s String) ENGINE = MergeTree ORDER BY id", tbl))
	require.NoError(t, err)

	strs := []string{`a'b\c`, "x\ny", `\N`}
	for i, s := range strs {
		q, args, err := gohan.Insert(tbl).Columns("id", "s").Values(i, s).Build(gohan.ClickHouseNamed())
		require.NoError(t, err)
		_, err = db.Exec(q, namedArgs(t, args, clickhouse.NanoSeconds)...)
		require.NoError(t, err)
	}

	for i, want := range strs {
		q, args, err := gohan.Select("s").From(tbl).Where(gohan.Col("id").Eq(i)).Build(gohan.ClickHouseNamed())
		require.NoError(t, err)
		var got string
		err = db.QueryRow(q, namedArgs(t, args, clickhouse.NanoSeconds)...).Scan(&got)
		require.NoError(t, err)
		require.Equal(t, want, got)
	}
}

// TestClickHouseNamedNativeAPI covers Step 5, point 4: the native
// clickhouse.Conn API's behaviour with the named-mode SQL gohan renders.
// clickhouse-go's native bind() recognizes only its own driver.NamedValue
// / driver.NamedDateValue types (see bind.go: checkAllNamedArguments),
// not database/sql's sql.NamedArg, so a raw sql.NamedArg is expected to be
// rejected; the observed error is asserted below rather than assumed.
func TestClickHouseNamedNativeAPI(t *testing.T) {
	dsn := chDSN(t)
	opts, err := clickhouse.ParseDSN(dsn)
	require.NoError(t, err)
	conn, err := clickhouse.Open(opts)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })

	ctx := context.Background()
	require.NoError(t, conn.Ping(ctx))

	tbl := "t_native_" + fmt.Sprint(tableSeq.Add(1))
	t.Cleanup(func() { _ = conn.Exec(context.Background(), "DROP TABLE IF EXISTS "+tbl) })
	require.NoError(t, conn.Exec(ctx, fmt.Sprintf(
		"CREATE TABLE %s (id UInt32, name String) ENGINE = MergeTree ORDER BY id", tbl)))

	q, args, err := gohan.Insert(tbl).Columns("id", "name").Values(1, "alice").Build(gohan.ClickHouseNamed())
	require.NoError(t, err)

	// Observed: passing the raw sql.NamedArg values gohan produced to the
	// native API's Exec fails (native bind() does not recognize
	// sql.NamedArg as a named argument type).
	rawArgs := make([]any, len(args))
	copy(rawArgs, args)
	err = conn.Exec(ctx, q, rawArgs...)
	require.Error(t, err, "expected the native API to reject raw sql.NamedArg; if this now succeeds, clickhouse-go's driver started recognizing sql.NamedArg and the docs recipe must be revisited")
	t.Logf("observed native API error for raw sql.NamedArg: %v", err)

	// Converted via clickhouse.Named(na.Name, na.Value): succeeds.
	converted := make([]any, len(args))
	for i, a := range args {
		na := a.(sql.NamedArg)
		converted[i] = clickhouse.Named(na.Name, na.Value)
	}
	require.NoError(t, conn.Exec(ctx, q, converted...))

	var name string
	require.NoError(t, conn.QueryRow(ctx, fmt.Sprintf("SELECT name FROM %s WHERE id = 1", tbl)).Scan(&name))
	require.Equal(t, "alice", name)
}
