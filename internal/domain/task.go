package domain

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

type (
	TaskID string
	UserID string
)

var (
	ErrTaskNotFound  = errors.New("task not found")
	ErrTaskClosed    = errors.New("task is already completed")
	ErrInvalidTask   = errors.New("invalid task")
	ErrInvalidFilter = errors.New("invalid filter")
	ErrInvalidPeriod = errors.New("invalid time period")
)

type Status int

const (
	StatusPending Status = iota
	StatusCompleted
	StatusRemoved
)

func (s Status) String() string {
	switch s {
	case StatusPending:
		return "PENDING"
	case StatusCompleted:
		return "COMPLETED"
	case StatusRemoved:
		return "REMOVED"
	default:
		return "UNKNOWN"
	}
}

type Task struct {
	ID          TaskID
	OwnerID     UserID
	Title       string
	Description string
	Tags        []string
	Deadline    time.Time // optional; zero = no deadline
	VisibleFrom time.Time // the task appears on the active TODO list from this instant
	Status      Status
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CompletedAt time.Time
	RemovedAt   time.Time
	Seq         uint64
}

func (t *Task) HasDeadline() bool { return !t.Deadline.IsZero() }

func (t *Task) IsActive(now time.Time) bool {
	return t.Status == StatusPending && !now.Before(t.VisibleFrom)
}

func (t *Task) IsUpcoming(now time.Time) bool {
	return t.Status == StatusPending && now.Before(t.VisibleFrom)
}

// ASSUMPTION: "spilled over" means the task is still pending after the deadline,
// or it was completed or removed after it deadline passed.
func (t *Task) SpilledOver(now time.Time) bool {
	if !t.HasDeadline() || !now.After(t.Deadline) {
		return false
	}
	closedAt := t.closedAt()
	return closedAt.IsZero() || closedAt.After(t.Deadline)
}

func (t *Task) closedAt() time.Time {
	switch t.Status {
	case StatusCompleted:
		return t.CompletedAt
	case StatusRemoved:
		return t.RemovedAt
	default:
		return time.Time{}
	}
}

// ASSUMPTION: tags arrive already normalised
func (t *Task) HasAllTags(want []string) bool {
	for _, tag := range want {
		if !slices.Contains(t.Tags, tag) {
			return false
		}
	}
	return true
}

func (t *Task) Clone() *Task {
	c := *t
	c.Tags = slices.Clone(t.Tags)
	return &c
}

func (t *Task) Validate() error {
	switch {
	case t.OwnerID == "":
		return fmt.Errorf("%w: owner is required", ErrInvalidTask)
	case strings.TrimSpace(t.Title) == "":
		return fmt.Errorf("%w: title is required", ErrInvalidTask)
	case t.HasDeadline() && t.Deadline.Before(t.VisibleFrom):
		return fmt.Errorf("%w: deadline %s is before visibleFrom %s",
			ErrInvalidTask, t.Deadline.Format(time.RFC3339), t.VisibleFrom.Format(time.RFC3339))
	}
	return nil
}

type EventType string

const (
	EventAdded     EventType = "ADDED"
	EventModified  EventType = "MODIFIED"
	EventCompleted EventType = "COMPLETED"
	EventRemoved   EventType = "REMOVED"
)

type ActivityEvent struct {
	At        time.Time
	Type      EventType
	TaskID    TaskID
	OwnerID   UserID
	TaskTitle string
}

func (e ActivityEvent) String() string {
	return fmt.Sprintf("%s  %-9s %s %q (owner: %s)",
		e.At.Format("2006-01-02 15:04"), e.Type, e.TaskID, e.TaskTitle, e.OwnerID)
}

// TimePeriod is the half-open interval [Start, End)
type TimePeriod struct {
	Start time.Time
	End   time.Time
}

func (p *TimePeriod) Validate() error {
	if p != nil && p.End.Before(p.Start) {
		return fmt.Errorf("%w: end %s is before start %s",
			ErrInvalidPeriod, p.End.Format(time.RFC3339), p.Start.Format(time.RFC3339))
	}
	return nil
}

func (p *TimePeriod) Contains(t time.Time) bool {
	return p == nil || (!t.Before(p.Start) && t.Before(p.End))
}

type Statistics struct {
	Added       int
	Completed   int
	SpilledOver int // tasks whose deadline fell in the period and was missed (see Task.SpilledOver)
}
