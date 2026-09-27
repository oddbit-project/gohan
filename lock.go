package gohan

import "fmt"

// lockWait selects the wait policy of a lockClause.
type lockWait int

const (
	lockWaitNone lockWait = iota
	lockWaitSkipLocked
	lockWaitNoWait
)

// lockClause is a single FOR UPDATE/SHARE clause.
type lockClause struct {
	strength string
	of       []string
	wait     lockWait
}

// addLock appends a lock clause of the given strength, copying s first
// (immutability).
func (s *SelectBuilder) addLock(strength string, of []string) *SelectBuilder {
	c := s.clone()
	c.locks = append(c.locks, lockClause{strength: strength, of: append([]string(nil), of...)})
	return c
}

// ForUpdate appends a FOR UPDATE clause, optionally naming the FROM items
// to lock with OF. PostgreSQL only; fails with ErrUnsupported on other
// dialects at Build.
func (s *SelectBuilder) ForUpdate(of ...string) *SelectBuilder {
	return s.addLock("FOR UPDATE", of)
}

// ForNoKeyUpdate appends a FOR NO KEY UPDATE clause, optionally naming the
// FROM items to lock with OF. PostgreSQL only; fails with ErrUnsupported on
// other dialects at Build.
func (s *SelectBuilder) ForNoKeyUpdate(of ...string) *SelectBuilder {
	return s.addLock("FOR NO KEY UPDATE", of)
}

// ForShare appends a FOR SHARE clause, optionally naming the FROM items to
// lock with OF. PostgreSQL only; fails with ErrUnsupported on other
// dialects at Build.
func (s *SelectBuilder) ForShare(of ...string) *SelectBuilder {
	return s.addLock("FOR SHARE", of)
}

// ForKeyShare appends a FOR KEY SHARE clause, optionally naming the FROM
// items to lock with OF. PostgreSQL only; fails with ErrUnsupported on
// other dialects at Build.
func (s *SelectBuilder) ForKeyShare(of ...string) *SelectBuilder {
	return s.addLock("FOR KEY SHARE", of)
}

// SkipLocked sets SKIP LOCKED on the most recently added lock clause.
// Calling it with no lock clause on the builder, or after NoWait was
// already set on the same clause, fails with ErrInvalidLock at Build.
func (s *SelectBuilder) SkipLocked() *SelectBuilder {
	c := s.clone()
	if len(c.locks) == 0 {
		c.lockErr = true
		return c
	}
	last := &c.locks[len(c.locks)-1]
	if last.wait != lockWaitNone {
		c.lockErr = true
		return c
	}
	last.wait = lockWaitSkipLocked
	return c
}

// NoWait sets NOWAIT on the most recently added lock clause. Calling it
// with no lock clause on the builder, or after SkipLocked was already set
// on the same clause, fails with ErrInvalidLock at Build.
func (s *SelectBuilder) NoWait() *SelectBuilder {
	c := s.clone()
	if len(c.locks) == 0 {
		c.lockErr = true
		return c
	}
	last := &c.locks[len(c.locks)-1]
	if last.wait != lockWaitNone {
		c.lockErr = true
		return c
	}
	last.wait = lockWaitNoWait
	return c
}

// renderLocks validates and renders every lock clause on s, in the "FOR
// UPDATE ... FOR SHARE ..." position after OFFSET and before SETTINGS.
func (s *SelectBuilder) renderLocks(w *writer) {
	if s.lockErr {
		w.fail(fmt.Errorf("%w: SkipLocked/NoWait requires a preceding lock clause and cannot be set twice on the same clause", ErrInvalidLock))
		return
	}
	if len(s.locks) == 0 {
		return
	}
	if !w.d.Has(FeatureLocking) {
		w.fail(fmt.Errorf("%w: FOR UPDATE/SHARE", ErrUnsupported))
		return
	}
	if s.distinct {
		w.fail(fmt.Errorf("%w: FOR UPDATE/SHARE with DISTINCT", ErrInvalidLock))
		return
	}
	if len(s.groupBy) > 0 {
		w.fail(fmt.Errorf("%w: FOR UPDATE/SHARE with GROUP BY", ErrInvalidLock))
		return
	}
	if len(s.having) > 0 {
		w.fail(fmt.Errorf("%w: FOR UPDATE/SHARE with HAVING", ErrInvalidLock))
		return
	}
	if len(s.unions) > 0 {
		w.fail(fmt.Errorf("%w: FOR UPDATE/SHARE with UNION/INTERSECT/EXCEPT", ErrInvalidLock))
		return
	}

	for _, lc := range s.locks {
		w.keyword(" " + lc.strength)
		if len(lc.of) > 0 {
			w.keyword(" OF ")
			for i, name := range lc.of {
				if i > 0 {
					w.keyword(", ")
				}
				writeAlias(w, name)
			}
		}
		switch lc.wait {
		case lockWaitSkipLocked:
			w.keyword(" SKIP LOCKED")
		case lockWaitNoWait:
			w.keyword(" NOWAIT")
		}
	}
}
