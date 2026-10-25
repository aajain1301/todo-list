package domain

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Scope selects which part of the task lifecycle a query lists.
// Removed tasks are never listed.
type Scope int

const (
	ScopePending Scope = iota
	ScopeCompleted
)

type Filter func(*Task) bool

type TaskFilter struct {
	Scope   Scope
	Filters []Filter
}

func (f TaskFilter) Validate() error {
	if f.Scope < ScopePending || f.Scope > ScopeCompleted {
		return fmt.Errorf("%w: unknown scope %d", ErrInvalidFilter, f.Scope)
	}
	if slices.ContainsFunc(f.Filters, func(x Filter) bool { return x == nil }) {
		return fmt.Errorf("%w: nil filter", ErrInvalidFilter)
	}
	return nil
}

func (f TaskFilter) Matcher() Filter {
	return func(t *Task) bool {
		for _, filter := range f.Filters {
			if !filter(t) {
				return false
			}
		}
		return true
	}
}

func Active(now time.Time) Filter {
	return func(t *Task) bool { return t.IsActive(now) }
}

func Upcoming(now time.Time) Filter {
	return func(t *Task) bool { return t.IsUpcoming(now) }
}

func HasTags(tags ...string) Filter {
	return func(t *Task) bool { return t.HasAllTags(tags) }
}

func DueBefore(d time.Time) Filter {
	return func(t *Task) bool { return t.HasDeadline() && t.Deadline.Before(d) }
}

func DueAfter(d time.Time) Filter {
	return func(t *Task) bool { return t.HasDeadline() && t.Deadline.After(d) }
}

func HasDeadline() Filter {
	return func(t *Task) bool { return t.HasDeadline() }
}

func TitleContains(text string) Filter {
	lower := strings.ToLower(text)
	return func(t *Task) bool { return strings.Contains(strings.ToLower(t.Title), lower) }
}

func Not(f Filter) Filter {
	return func(t *Task) bool { return !f(t) }
}

func Or(fs ...Filter) Filter {
	return func(t *Task) bool {
		for _, f := range fs {
			if f(t) {
				return true
			}
		}
		return false
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
