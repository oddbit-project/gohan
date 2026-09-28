package gohan

type Error string

func (e Error) Error() string {
	return string(e)
}

const (
	ErrUnknownDialect    = Error("gohan: unknown or zero dialect")
	ErrInvalidIdentifier = Error("gohan: invalid identifier")
	ErrUnsupported       = Error("gohan: not supported by dialect")
	ErrRawArgs           = Error("gohan: raw placeholder count does not match arguments")
	ErrRawPlaceholder    = Error("gohan: raw sql contains a forbidden placeholder sequence")
	ErrInvalidFunction   = Error("gohan: invalid function name")
	ErrInvalidType       = Error("gohan: invalid cast type")
	ErrEmptyMatch        = Error("gohan: Match requires at least one field")
	ErrEmptyCase         = Error("gohan: CASE requires at least one WHEN")
	ErrUnsafeValue       = Error("gohan: value type cannot be bound safely for this dialect")
	ErrTooManyArgs       = Error("gohan: statement exceeds the dialect's bound-argument limit")
	ErrNilExpr           = Error("gohan: nil or zero expression")
	ErrNoWhere           = Error("gohan: statement requires a WHERE clause; call All() to affect every row")
	ErrNoTable           = Error("gohan: statement has no table")
	ErrNeedAlias         = Error("gohan: subquery in FROM requires an alias")
	ErrInvalidColumn     = Error("gohan: column must be a string or an expression")
	ErrInvalidLimit      = Error("gohan: LIMIT/OFFSET out of range for dialect")

	ErrNoColumns        = Error("gohan: statement has no columns to write")
	ErrValueCount       = Error("gohan: value count does not match column count")
	ErrInsertMixed      = Error("gohan: Rows, Values, SetMap, FromSelect and DefaultValues cannot be combined")
	ErrInvalidRecord    = Error("gohan: record must be a non-nil struct or pointer to struct")
	ErrRecordType       = Error("gohan: all records must have the same type")
	ErrRecordShape      = Error("gohan: record type has a field shape gohan cannot map safely")
	ErrInconsistentOmit = Error("gohan: records disagree on omitted fields")
	ErrUnknownField     = Error("gohan: field not found in record")
	ErrDuplicateColumn  = Error("gohan: column set more than once")
	ErrConflictTarget   = Error("gohan: DO UPDATE requires conflict columns")

	ErrCompoundPart   = Error("gohan: a UNION member cannot have ORDER BY, LIMIT, OFFSET, WITH, SETTINGS, its own UNION or a row-locking clause")
	ErrInvalidSample  = Error("gohan: SAMPLE ratio must be in (0, 1]")
	ErrInvalidSetting = Error("gohan: invalid SETTINGS name")
	ErrEmptyClause    = Error("gohan: clause requires at least one column")
	ErrInvalidLock    = Error("gohan: invalid row-locking clause")
)
