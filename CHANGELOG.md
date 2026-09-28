# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

## [v0.4.0] - 2026-09-28

### Changed

- **Breaking:** `Eq`, `Neq` and `Val` now render `IS NULL`/`IS NOT NULL`/`NULL` only for an
  untyped `nil`. A nil pointer (e.g. `var p *int; Col("x").Eq(p)`) is no longer treated as nil: it
  is bound as an ordinary parameter, so the driver sends `NULL` and the comparison matches no row.

  ```go
  var p *int // nil

  gohan.Col("x").Eq(p)
  // before: "x" IS NULL        -> matches every NULL row
  // after:  "x" = $1 (NULL)    -> matches no row

  gohan.Col("x").Neq(p)
  // before: "x" IS NOT NULL    -> matches every non-NULL row
  // after:  "x" <> $1 (NULL)   -> matches no row
  ```

  To test for NULL explicitly, use `Eq(nil)`/`IsNull()` or `Neq(nil)`/`IsNotNull()` with an
  untyped `nil`. `Match` inherits this through `Eq`.

  `Val(p)` with a nil pointer is likewise bound as a parameter rather than rendering the `NULL`
  keyword, so it no longer has a literal for `IsNull()`/`IsNotNull()` to attach to —
  `Val(p).IsNull()` renders `$1 IS NULL`, which PostgreSQL rejects (`42P18`). Use `Val(nil)` for a
  NULL literal.

### Fixed

- A nil pointer whose type implements `driver.Valuer` is bound as untyped `nil` on every dialect,
  not just ClickHouse. Without this, binding such a value as an ordinary parameter — which
  `Eq`/`Neq`/`Val`/`Set` now do for any nil pointer — would let database/sql call a
  pointer-receiver `Value()` method on the nil pointer and panic (clickhouse-go calls `Value()` for
  either receiver kind). Note: a pointer-receiver `Valuer` that deliberately returns a non-NULL
  value for a nil receiver is now bound as NULL.

## [v0.3.0] - 2026-09-28

### Added

- `Value.ContainsFold`, `HasPrefixFold` and `HasSuffixFold`: case-insensitive forms of `Contains`,
  `HasPrefix` and `HasSuffix`, with the same escaping of `%`, `_` and the escape character. They
  render `ILIKE` on PostgreSQL and ClickHouse (`FeatureILike`) and `LIKE` on SQLite, whose `LIKE`
  folds ASCII letters only; they fail with `ErrUnsupported` on Generic. See
  [LIKE and pattern helpers](docs/expressions.md#like-and-pattern-helpers).

## [v0.2.0] - 2026-09-28

### Added

- `integration/`: a separate Go module that runs gohan's rendered SQL against real PostgreSQL,
  SQLite and ClickHouse servers (round-tripped values, LIKE escaping, UNION/ORDER BY/LIMIT,
  upsert, DELETE, quoted identifiers), run by `make integration` and a dedicated CI job.
- `docs/`: topic guides (getting started, SELECT, INSERT and upsert, UPDATE and DELETE,
  expressions, raw SQL and escape hatches, dialects, records and struct tags, security model,
  errors), with every SQL string and argument list taken from real output.
- Runnable godoc examples for most exported functions and methods of `gohan` and `field`, checked
  by `go test`.
- `examples/`: a separate Go module with end-to-end programs for SQLite (`modernc.org/sqlite`),
  PostgreSQL (`pgx` stdlib) and ClickHouse (`clickhouse-go` v2); `gohan`'s own dependencies are
  unchanged.
- README badges (pkg.go.dev, test and security workflows, release, license, Go version) and links
  to the docs and examples.
- CI and `make test` vet and build the examples module and run the SQLite example.
- `fuzz` workflow: runs each fuzz target (`FuzzIdentRoundTrip`, `FuzzValueNeverInlined`,
  `FuzzRawNoOrdinal`) for 60s on pushes to main and pull requests, and for 10 minutes weekly;
  failing inputs are uploaded as an artifact. `make fuzz` runs them locally.
- Documentation site at https://oddbit-project.github.io/gohan/, built from `docs/` with MkDocs
  (`mkdocs.yml`, pinned in `requirements-docs.txt`) and deployed by the `docs` workflow on pushes
  to main.
- `gohan.ClickHouseNamed()`: a ClickHouse dialect variant with `@pN` named placeholders and
  `sql.NamedArg` arguments instead of `?` and bare values. Convert a `sql.NamedArg`'s `time.Time`
  value to `clickhouse.DateNamed(name, t, scale)` before executing — the only way to keep
  `DateTime64` sub-second precision through clickhouse-go. Not added to the default driver
  registry. See [Named parameters](docs/dialects.md#named-parameters).
- Row-locking clauses on `SelectBuilder`: `ForUpdate`, `ForNoKeyUpdate`, `ForShare`, `ForKeyShare`
  (each with optional `OF` names), and `SkipLocked`/`NoWait`. PostgreSQL only (`FeatureLocking`);
  fails with `ErrUnsupported` elsewhere, and with `ErrInvalidLock` combined with `DISTINCT`,
  `GROUP BY`/`HAVING` or `UNION`. See [Row locking](docs/select.md#row-locking).
- `IsEmpty(cond)` and `IsTrivial(cond)`: exported checks for whether a condition places no
  restriction at all, including a column-free constant such as `Raw("1=1")`, `Raw("true")` or
  `Val(1).Eq(1)` that the builders could not previously detect. See
  [Checking a condition](docs/expressions.md#checking-a-condition).
- `InsertBuilder.DefaultValues()`: renders `INSERT INTO t DEFAULT VALUES`, for a row that takes
  every column's default. Cannot be combined with `Columns`, `Values`, `Rows`, `SetMap` or
  `FromSelect` (`ErrInsertMixed`). `FeatureDefaultValues`: PostgreSQL, SQLite and Generic; fails
  with `ErrUnsupported` on ClickHouse (real ClickHouse rejects it with a syntax error), and also
  with `ErrUnsupported` combined with `OnConflict` on SQLite (a syntax error there too; PostgreSQL
  allows it). See [DEFAULT VALUES](docs/insert-and-upsert.md#default-values).

### Changed

- `Update`/`Delete`'s `WHERE` guard now uses `IsTrivial`, so a column-free condition — for example
  `Where(Raw("1=1"))` or `Where(Val(1).Eq(1))` — fails with `ErrNoWhere` the same as an empty or
  always-true one, unless `All()` is called. This is stricter than v0.1.0: code that relied on a
  constant `Raw` condition to affect every row now needs `All()`. See
  [The WHERE requirement](docs/update-and-delete.md#the-where-requirement).

### Fixed

- README: `Case`'s `WHEN` is a condition, not a value position; the alias tag precedence is
  `alias`, `json`, `xml`; a column mapped by two fields usually fails with `ErrRecordShape`, not
  `ErrDuplicateColumn`; the ClickHouse dialect's lack of `UPDATE` is a `gohan` limitation, since
  recent ClickHouse versions have a lightweight `UPDATE`.
- Documented (not fixed — no `gohan`-side fix is possible): the default `ClickHouse()` dialect's
  positional `?` binding truncates a bound `time.Time` to seconds precision through clickhouse-go,
  silently losing the sub-second part of a `DateTime64(3/6/9)` column on insert and in `WHERE`
  comparisons (measured against clickhouse-go v2.40.3 and v2.48.0). Use `ClickHouseNamed()` to
  keep full precision.

## [v0.1.0] - 2026-09-27

### Added

- Initial release: extracted from `oddbit-project/blueprint` `sqlb`.
- `SECURITY.md`: vulnerability reporting through GitHub private vulnerability reporting.
- CI security workflow: Trivy repository and SBOM scans (CRITICAL/HIGH fail the build, results in
  code scanning, weekly re-scan) and a CycloneDX SBOM attached to tagged releases; `make sbom` /
  `make scan` for local runs.

### Changed

- Root package renamed `sqlb` → `gohan`; error strings' prefix changed from `"sqlb: "` to
  `"gohan: "`.
- Struct-metadata helpers moved from Blueprint's `db/field` into this module's own `field`
  subpackage, with no dependency on Blueprint.
