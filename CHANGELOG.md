# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Added

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

### Fixed

- README: `Case`'s `WHEN` is a condition, not a value position; the alias tag precedence is
  `alias`, `json`, `xml`; a column mapped by two fields usually fails with `ErrRecordShape`, not
  `ErrDuplicateColumn`; the ClickHouse dialect's lack of `UPDATE` is a `gohan` limitation, since
  recent ClickHouse versions have a lightweight `UPDATE`.

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
