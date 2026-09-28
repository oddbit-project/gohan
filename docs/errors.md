# Errors

Builder methods never return errors. Every problem is recorded and returned by `Build` (and by
`Dialect.QuoteIdent`, `DialectFor`, `RecordColumns` and `InsertColumns`). When `Build` fails it
returns an empty SQL string and no arguments. Only the first error is reported.

Each error is one of the `gohan.Err*` constants below, often wrapped with detail such as the
offending name. Compare with `errors.Is`, not `==`:

```go
_, _, err := gohan.Select("id").From("t").Where(gohan.Col("a").ILike("x%")).Build(gohan.SQLite())
fmt.Println(err == gohan.ErrUnsupported, errors.Is(err, gohan.ErrUnsupported))
fmt.Println(err)
// Output:
// false true
// gohan: not supported by dialect: ILIKE
```

## Reference

### Dialect and identifiers

| Error | Returned when |
|---|---|
| `ErrUnknownDialect` | `DialectFor` gets an unregistered name, or `Build`/`QuoteIdent` gets the zero `Dialect{}` |
| `ErrInvalidIdentifier` | an identifier is empty, has an empty dotted part or a NUL byte; on ClickHouse, contains `?`, `@`, `{`, `}` or `$` followed by a digit; an alias or `USING` column is dotted or `*`; a CTE name or a written column (`INSERT`/`UPDATE`/conflict target) is dotted |
| `ErrUnsupported` | a clause or feature the dialect lacks: `ILIKE` (including `ContainsFold`/`HasPrefixFold`/`HasSuffixFold` on Generic), `RETURNING`, `ON CONFLICT`, `Excluded`, `UPDATE`, the ClickHouse-only clauses; also `Final`/`Sample` on a subquery, more than one `ArrayJoin`/`LeftArrayJoin` call on a builder, an aliased `INSERT`/`UPDATE` table, an aliased ClickHouse `DELETE` table |

### Statements

| Error | Returned when |
|---|---|
| `ErrNoTable` | the table is missing, empty, or of the wrong type; `Select()` with neither columns nor `From` |
| `ErrNeedAlias` | a subquery used as a `FROM`/`JOIN` source without `As(alias)` |
| `ErrInvalidColumn` | a select-list, `GROUP BY` or `RETURNING` item that is neither a string nor an expression; an `ORDER BY` item that is not a string, `Value` or `Order`; a statement builder, `TableRef` or `Subquery` passed as a value |
| `ErrNoWhere` | `UPDATE`/`DELETE` without a `WHERE`, or with one that is always true, and without `All()` |
| `ErrInvalidLimit` | `LIMIT`/`OFFSET` above `math.MaxInt64` (not on ClickHouse) |
| `ErrCompoundPart` | a `UNION` member with its own `ORDER BY`, `LIMIT`, `OFFSET`, `WITH`, `SETTINGS`, `UNION` or a row-locking clause |
| `ErrEmptyClause` | `ArrayJoin`/`LeftArrayJoin`, `JoinUsing`/`LeftJoinUsing` with no columns |
| `ErrInvalidSample` | `Sample` ratio outside (0, 1] or not finite; `SampleRows(0)` |
| `ErrInvalidSetting` | a `Settings` key not matching `^[A-Za-z_][A-Za-z0-9_]*$` |
| `ErrTooManyArgs` | more bound arguments than the dialect's limit |
| `ErrInvalidLock` | `ForUpdate`/`ForShare`/etc. combined with `Distinct`, `GroupBy`, `Having` or `UNION`; `SkipLocked`/`NoWait` with no preceding lock clause, or set twice on the same clause |

### Writes

| Error | Returned when |
|---|---|
| `ErrNoColumns` | nothing to write: no row source, `Rows()` with no records, an empty `SetMap`, `FromSelect` without `Columns`, a record with no insertable field, an `UPDATE` with no assignments, an empty `DoUpdate` map or `DoUpdateExcluded()` with no columns |
| `ErrValueCount` | a `Values` row whose length differs from `Columns`, or `Values` without `Columns` |
| `ErrInsertMixed` | more than one of `Values`, `Rows`, `SetMap`, `FromSelect` |
| `ErrConflictTarget` | `DoUpdate`/`DoUpdateExcluded` without conflict columns |
| `ErrUnknownField` | a `DoUpdateExcluded` column that is not inserted; an `IncludeFields`/`ExcludeFields` name that matches no field |
| `ErrDuplicateColumn` | a column assigned twice in an `UPDATE`; see also [Records](records-and-struct-tags.md#errors) |

### Records

| Error | Returned when |
|---|---|
| `ErrInvalidRecord` | a record that is nil, a nil pointer, or not a struct |
| `ErrRecordType` | `Rows` records of different struct types |
| `ErrRecordShape` | a struct layout that cannot be mapped unambiguously; see [Records](records-and-struct-tags.md#embedded-structs) |
| `ErrInconsistentOmit` | `Rows` records that disagree on which `omitnil`/`omitempty` fields are present |

### Expressions and escape hatches

| Error | Returned when |
|---|---|
| `ErrNilExpr` | a nil or zero-value expression, including a nil `*Value`/`*Order`, a nil subquery, a join without an `ON` condition |
| `ErrRawArgs` | the number of `?` markers in `Raw` differs from the number of arguments |
| `ErrRawPlaceholder` | `Raw` text contains a sequence a driver could read as a placeholder; see [Raw](raw-and-escape-hatches.md#rejected-placeholder-sequences) |
| `ErrInvalidFunction` | an `Fn` name not matching `^[A-Za-z_][A-Za-z0-9_]*$` |
| `ErrInvalidType` | a `Cast` type that does not start with a letter or `_`, has characters outside letters, digits, `_`, space, `,`, `()`, `[]`, or has unbalanced parentheses |
| `ErrEmptyMatch` | `Match` with a nil or empty map |
| `ErrEmptyCase` | `Case()` finished without any `When` |
| `ErrUnsafeValue` | on ClickHouse, a value clickhouse-go cannot bind safely; see [Dialects](dialects.md#values) |
