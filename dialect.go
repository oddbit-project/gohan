package gohan

import (
	"fmt"
	"sync"
)

// Feature is a bitmask of optional SQL capabilities a Dialect supports.
type Feature uint

const (
	FeatureReturning  Feature = 1 << iota // RETURNING on INSERT/UPDATE/DELETE
	FeatureUpsert                         // INSERT ... ON CONFLICT
	FeatureILike                          // ILIKE operator
	FeatureUpdate                         // UPDATE statement
	FeatureClickHouse                     // FINAL, PREWHERE, SAMPLE, ARRAY JOIN, SETTINGS
)

// quoteKind selects the identifier-quoting rules for a Dialect.
type quoteKind uint

const (
	quoteDouble quoteKind = iota
	quoteBacktick
	quoteClickHouse
)

// Dialect describes the SQL identifier quoting, placeholder style and
// feature set of a target database.
type Dialect struct {
	name     string
	quote    quoteKind
	features Feature
	maxArgs  int
	named    bool
}

// Postgres returns the PostgreSQL dialect: numbered ($n) placeholders,
// double-quoted identifiers, RETURNING/upsert/ILIKE/UPDATE support, and a
// 65535 bound-argument limit.
func Postgres() Dialect {
	return Dialect{
		name:     "postgres",
		quote:    quoteDouble,
		features: FeatureReturning | FeatureUpsert | FeatureILike | FeatureUpdate,
		maxArgs:  65535,
	}
}

// SQLite returns the SQLite dialect: `?` placeholders, backtick-quoted
// identifiers, RETURNING/upsert/UPDATE support, and a 32766 bound-argument
// limit.
func SQLite() Dialect {
	return Dialect{
		name:     "sqlite",
		quote:    quoteBacktick,
		features: FeatureReturning | FeatureUpsert | FeatureUpdate,
		maxArgs:  32766,
	}
}

// ClickHouse returns the ClickHouse dialect: `?` placeholders,
// double-quoted identifiers with backslash doubling, ILIKE/ClickHouse
// clause support, and no bound-argument limit.
//
// clickhouse-go formats a bound time.Time at seconds precision when
// binding positional `?` placeholders (measured against clickhouse-go
// v2.40.3 and v2.48.0), so a DateTime64(3/6/9) column silently loses its
// sub-second part on insert and in comparisons. Use ClickHouseNamed to
// keep full precision.
func ClickHouse() Dialect {
	return Dialect{
		name:     "clickhouse",
		quote:    quoteClickHouse,
		features: FeatureILike | FeatureClickHouse,
		maxArgs:  0,
	}
}

// ClickHouseNamed returns the ClickHouse dialect with named placeholders: each
// bound value is written as @p1, @p2, … and returned as a
// sql.NamedArg{Name: "p1", Value: v}. Use it to pass time values as
// clickhouse.DateNamed (the only way to keep DateTime64 sub-second
// precision with clickhouse-go, which formats time.Time at seconds
// precision through positional binding — see ClickHouse). It is not added
// to the default registry; call Register if you want it looked up by
// driver name.
//
//	q, args, _ := st.Build(gohan.ClickHouseNamed())
//	for i, a := range args {
//	    na := a.(sql.NamedArg)
//	    if t, ok := na.Value.(time.Time); ok {
//	        args[i] = clickhouse.DateNamed(na.Name, t, clickhouse.NanoSeconds)
//	    }
//	}
//	rows, err := db.QueryContext(ctx, q, args...)
//
// For clickhouse-go's native API (clickhouse.Conn), convert every
// sql.NamedArg to clickhouse.Named(na.Name, na.Value) instead — the native
// API does not understand sql.NamedArg.
func ClickHouseNamed() Dialect {
	return Dialect{
		name:     "clickhouse-named",
		quote:    quoteClickHouse,
		features: FeatureILike | FeatureClickHouse,
		maxArgs:  0,
		named:    true,
	}
}

// Generic returns the ANSI SQL dialect: `?` placeholders, ANSI
// double-quoted identifiers, UPDATE support, and a 999 bound-argument
// limit. It must not be registered for MySQL, where double-quoted text is
// a string literal, not an identifier.
func Generic() Dialect {
	return Dialect{
		name:     "generic",
		quote:    quoteDouble,
		features: FeatureUpdate,
		maxArgs:  999,
	}
}

// Name returns the dialect's name, or "" for the zero Dialect.
func (d Dialect) Name() string {
	return d.name
}

// Has reports whether the dialect supports the given feature.
func (d Dialect) Has(f Feature) bool {
	return d.features&f != 0
}

// MaxArgs returns the dialect's bound-argument limit, or 0 for unlimited.
func (d Dialect) MaxArgs() int {
	return d.maxArgs
}

var (
	registryMu sync.RWMutex
	registry   = map[string]Dialect{
		"pgx":        Postgres(),
		"pgx/v5":     Postgres(),
		"postgres":   Postgres(),
		"sqlite":     SQLite(),
		"sqlite3":    SQLite(),
		"clickhouse": ClickHouse(),
	}
)

// Register associates driverName with d, overriding any previous
// registration.
func Register(driverName string, d Dialect) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[driverName] = d
}

// DialectFor returns the Dialect registered for driverName, or
// ErrUnknownDialect if none is registered.
func DialectFor(driverName string) (Dialect, error) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	d, ok := registry[driverName]
	if !ok {
		return Dialect{}, fmt.Errorf("%w: %q", ErrUnknownDialect, driverName)
	}
	return d, nil
}
