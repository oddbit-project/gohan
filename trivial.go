package gohan

import "strings"

// rawKeywords are words that are not identifiers in Raw SQL text, even
// though they match the identifier pattern.
var rawKeywords = map[string]bool{
	"TRUE": true, "FALSE": true, "NULL": true, "NOT": true, "AND": true,
	"OR": true, "IS": true, "DISTINCT": true, "FROM": true, "BETWEEN": true,
	"IN": true, "LIKE": true, "ILIKE": true,
}

func isIdentStart(b byte) bool {
	return b == '_' || (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

func isIdentPart(b byte) bool {
	return isIdentStart(b) || isDigit(b)
}

// rawIdentifierCount scans sql (trusted Raw text, already accepted by
// validateRaw) and counts tokens that look like column/table references:
// bare words other than a small operator-keyword allowlist, and any
// double-quoted or backtick-quoted token. Single-quoted string literals
// (with ” escapes) and numbers are skipped, not counted.
func rawIdentifierCount(sql string) int {
	count := 0
	n := len(sql)
	i := 0
	for i < n {
		c := sql[i]
		switch {
		case c == '\'':
			// Skip a single-quoted string literal, handling '' escapes.
			i++
			for i < n {
				if sql[i] == '\'' {
					if i+1 < n && sql[i+1] == '\'' {
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
		case c == '"' || c == '`':
			quote := c
			i++
			for i < n {
				if sql[i] == quote {
					if i+1 < n && sql[i+1] == quote {
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
			count++
		case isDigit(c):
			i++
			for i < n && (isDigit(sql[i]) || sql[i] == '.') {
				i++
			}
		case isIdentStart(c):
			start := i
			i++
			for i < n && isIdentPart(sql[i]) {
				i++
			}
			word := sql[start:i]
			if !rawKeywords[strings.ToUpper(word)] {
				count++
			}
		default:
			i++
		}
	}
	return count
}

// IsEmpty reports whether cond places no condition at all: nil, the zero
// Value, a nil *Value/*Order, or And() with no elements (directly or
// nested only in other empty Ands).
func IsEmpty(cond Expr) bool {
	ne, valid := normExpr(cond)
	if !valid {
		return true
	}
	v, ok := ne.(Value)
	if !ok {
		return false
	}
	if v.fn == nil {
		return true
	}
	return v.empty
}

// IsTrivial reports whether cond does not restrict rows: it IsEmpty, it is
// syntactically always true (the existing trivial flag), or it references
// no column at all — a constant such as Raw("1=1"), Raw("true"),
// Raw("? = ?", 1, 1) or Val(1).Eq(1).
//
// Known limits (not detected): a column compared with itself
// (Col("id").Eq(Col("id"))), an uncorrelated subquery (its referenced
// table still counts as a column), and a Raw function call on constants
// (Raw("abs(1) = 1") — "abs" counts as an identifier).
func IsTrivial(cond Expr) bool {
	if IsEmpty(cond) {
		return true
	}
	// IsEmpty(cond) was false, so cond normalizes to a valid Expr.
	ne, _ := normExpr(cond)
	if v, ok := ne.(Value); ok && v.trivial {
		return true
	}
	w := &writer{d: Generic()}
	ne.render(w)
	if w.err != nil {
		return false
	}
	return w.idents == 0 && w.rawIdents == 0
}
