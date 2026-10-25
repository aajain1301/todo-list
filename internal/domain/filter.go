package domain

import (
	"cmp"
	"fmt"
	"time"
)

// Scope selects which part of the task lifecycle a query lists.
// Removed tasks are never listed.
type Scope int

const (
	ScopeActive Scope = iota
	ScopeUpcoming
	ScopePending
	ScopeCompleted
)

type TaskFilter struct {
	Scope Scope
	Tags  []string
}

func (f TaskFilter) Validate() error {
	if f.Scope < ScopeActive || f.Scope > ScopeCompleted {
		return fmt.Errorf("%w: unknown scope %d", ErrInvalidFilter, f.Scope)
	}
	return nil
}

func (f TaskFilter) Matcher(now time.Time) func(*Task) bool {
	inScope := f.scopePredicate(now)
	return func(t *Task) bool { return inScope(t) && t.HasAllTags(f.Tags) }
}

func (f TaskFilter) scopePredicate(now time.Time) func(*Task) bool {
	switch f.Scope {
	case ScopeUpcoming:
		return func(t *Task) bool { return t.IsUpcoming(now) }
	case ScopePending:
		return func(t *Task) bool { return t.Status == StatusPending }
	case ScopeCompleted:
		return func(t *Task) bool { return t.Status == StatusCompleted }
	default:
		return func(t *Task) bool { return t.IsActive(now) }
	}
}

func CompareByDeadline(a, b *Task) int {
	if a.HasDeadline() != b.HasDeadline() {
		if a.HasDeadline() {
			return -1
		}
		return 1
	}
	if c := a.Deadline.Compare(b.Deadline); c != 0 {
		return c
	}
	return cmp.Compare(a.Seq, b.Seq)
}
