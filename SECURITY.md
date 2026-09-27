# Security Policy

## Supported versions

Gohan is pre-1.0. Security fixes are released for the **latest minor version** only; please
upgrade to the newest release before reporting.

| Version | Supported |
|---------|-----------|
| latest `v0.x` release | yes |
| older releases | no |

## Reporting a vulnerability

**Do not open a public issue, discussion or pull request for a security problem.**

Report it privately through GitHub:
[Report a vulnerability](https://github.com/oddbit-project/gohan/security/advisories/new)
(the **Security** tab → **Report a vulnerability**).

Please include:

- the affected version (or commit) and dialect (PostgreSQL, SQLite, ClickHouse, Generic);
- the database driver and its version (e.g. `pgx`, `modernc.org/sqlite`, `clickhouse-go`);
- a minimal code sample that builds the query, the SQL and arguments it produces, and what
  goes wrong (for example: caller-controlled text reaching the SQL outside a bound argument or a
  quoted identifier);
- any known workaround.

We will acknowledge the report, work on a fix in a private advisory, and publish the advisory
together with the patched release. Reporters are credited in the advisory unless they ask not
to be.

## What is in scope

Gohan's contract is that **values are always bound and identifiers are always quoted and
escaped**. Anything that lets caller-controlled data escape that contract through the normal API
is in scope — including dialect-specific escaping, placeholder handling, and the ClickHouse
value checks (which depend on how `clickhouse-go` formats bound values client-side).

Not vulnerabilities in gohan (by design, documented in the README):

- SQL text passed to `Raw`, function names passed to `Fn`, and type names passed to `Cast` —
  these are trusted-input escape hatches and must never be built from request data;
- column or table names taken from request data without an allowlist (quoting prevents syntax
  injection, not access to columns you did not intend to expose);
- behaviour of database drivers or servers themselves — please report those upstream (we still
  appreciate a heads-up if gohan should defend against it).
