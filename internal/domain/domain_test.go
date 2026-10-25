package domain_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"todolist/internal/domain"
)

var base = time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)

func hoursAfter(h int) time.Time { return base.Add(time.Duration(h) * time.Hour) }

func TestStatus_String(t *testing.T) {
	cases := map[domain.Status]string{
		domain.StatusPending:   "PENDING",
		domain.StatusCompleted: "COMPLETED",
		domain.StatusRemoved:   "REMOVED",
		domain.Status(99):      "UNKNOWN",
	}
	for status, want := range cases {
		if got := status.String(); got != want {
			t.Errorf("Status(%d).String() = %q, want %q", int(status), got, want)
		}
	}
}

func TestTask_HasDeadline(t *testing.T) {
	if (&domain.Task{}).HasDeadline() {
		t.Error("zero deadline should mean no deadline")
	}
	if !(&domain.Task{Deadline: base}).HasDeadline() {
		t.Error("task with a deadline should report it")
	}
}

func TestTask_IsActive(t *testing.T) {
	task := &domain.Task{Status: domain.StatusPending, VisibleFrom: hoursAfter(2)}
	if task.IsActive(hoursAfter(1)) {
		t.Error("task should not be active before VisibleFrom")
	}
	if !task.IsActive(hoursAfter(2)) {
		t.Error("task should be active from VisibleFrom")
	}
}

func TestTask_IsUpcoming(t *testing.T) {
	task := &domain.Task{Status: domain.StatusPending, VisibleFrom: hoursAfter(2)}
	if !task.IsUpcoming(hoursAfter(1)) {
		t.Error("task should be upcoming before VisibleFrom")
	}
	if task.IsUpcoming(hoursAfter(2)) {
		t.Error("task should not be upcoming once visible")
	}
}

func TestTask_SpilledOver(t *testing.T) {
	deadline := hoursAfter(2)
	now := hoursAfter(5)
	cases := []struct {
		name string
		task domain.Task
		want bool
	}{
		{"completed late", domain.Task{Deadline: deadline, Status: domain.StatusCompleted, CompletedAt: hoursAfter(3)}, true},
		{"completed exactly at deadline", domain.Task{Deadline: deadline, Status: domain.StatusCompleted, CompletedAt: deadline}, false},
		{"still pending after deadline", domain.Task{Deadline: deadline, Status: domain.StatusPending}, true},
		{"removed before deadline", domain.Task{Deadline: deadline, Status: domain.StatusRemoved, RemovedAt: hoursAfter(1)}, false},
		{"deadline not reached yet", domain.Task{Deadline: hoursAfter(9), Status: domain.StatusPending}, false},
		{"no deadline", domain.Task{Status: domain.StatusPending}, false},
	}
	for _, tc := range cases {
		if got := tc.task.SpilledOver(now); got != tc.want {
			t.Errorf("%s: SpilledOver = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestTask_HasAllTags(t *testing.T) {
	task := &domain.Task{Tags: []string{"home", "urgent", "work"}}
	if !task.HasAllTags([]string{"urgent", "work"}) {
		t.Error("task should have all of [urgent work]")
	}
	if task.HasAllTags([]string{"urgent", "school"}) {
		t.Error("task should not match when one tag is missing")
	}
}

func TestTask_Clone(t *testing.T) {
	original := &domain.Task{ID: "T-1", Tags: []string{"a", "b"}}
	clone := original.Clone()
	clone.Tags[0] = "changed"
	if original.Tags[0] != "a" {
		t.Errorf("changing the clone's tags changed the original: %v", original.Tags)
	}
}

func TestTask_Validate(t *testing.T) {
	valid := domain.Task{OwnerID: "alice", Title: "Write report", VisibleFrom: base, Deadline: hoursAfter(1)}
	cases := []struct {
		name    string
		mutate  func(*domain.Task)
		wantErr bool
	}{
		{"valid", func(*domain.Task) {}, false},
		{"missing owner", func(t *domain.Task) { t.OwnerID = "" }, true},
		{"blank title", func(t *domain.Task) { t.Title = "   " }, true},
		{"deadline before visibleFrom", func(t *domain.Task) { t.Deadline = base.Add(-time.Minute) }, true},
	}
	for _, tc := range cases {
		task := valid
		tc.mutate(&task)
		err := task.Validate()
		if tc.wantErr && !errors.Is(err, domain.ErrInvalidTask) {
			t.Errorf("%s: err = %v, want ErrInvalidTask", tc.name, err)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("%s: unexpected error %v", tc.name, err)
		}
	}
}

func TestActivityEvent_String(t *testing.T) {
	event := domain.ActivityEvent{At: base, Type: domain.EventAdded, TaskID: "T-1", OwnerID: "alice", TaskTitle: "Write report"}
	got := event.String()
	for _, part := range []string{"2026-01-01 09:00", "ADDED", "T-1", `"Write report"`, "alice"} {
		if !strings.Contains(got, part) {
			t.Errorf("String() = %q, missing %q", got, part)
		}
	}
}

func TestTimePeriod_Validate(t *testing.T) {
	var allTime *domain.TimePeriod
	if err := allTime.Validate(); err != nil {
		t.Errorf("nil period should be valid, got %v", err)
	}
	inverted := &domain.TimePeriod{Start: hoursAfter(2), End: hoursAfter(1)}
	if err := inverted.Validate(); !errors.Is(err, domain.ErrInvalidPeriod) {
		t.Errorf("inverted period: err = %v, want ErrInvalidPeriod", err)
	}
}

func TestTimePeriod_Contains(t *testing.T) {
	period := &domain.TimePeriod{Start: hoursAfter(1), End: hoursAfter(2)}
	if !period.Contains(hoursAfter(1)) {
		t.Error("start should be included")
	}
	if period.Contains(hoursAfter(2)) {
		t.Error("end should be excluded")
	}
	var allTime *domain.TimePeriod
	if !allTime.Contains(hoursAfter(100)) {
		t.Error("nil period should contain every time")
	}
}

func TestTaskFilter_Validate(t *testing.T) {
	if err := (domain.TaskFilter{}).Validate(); err != nil {
		t.Errorf("zero filter should be valid, got %v", err)
	}
	if err := (domain.TaskFilter{Scope: 42}).Validate(); !errors.Is(err, domain.ErrInvalidFilter) {
		t.Errorf("unknown scope: err = %v, want ErrInvalidFilter", err)
	}
}

func TestTaskFilter_Matcher(t *testing.T) {
	now := hoursAfter(1)
	task := func(id domain.TaskID, visibleFrom time.Time, tags ...string) *domain.Task {
		return &domain.Task{ID: id, Status: domain.StatusPending, VisibleFrom: visibleFrom, Tags: tags}
	}
	tasks := []*domain.Task{
		task("match", base, "urgent", "work"),
		task("upcoming", hoursAfter(5), "work"),
		task("other-tag", base, "home"),
	}

	matches := domain.TaskFilter{Tags: []string{"work"}}.Matcher(now)

	var got []domain.TaskID
	for _, tk := range tasks {
		if matches(tk) {
			got = append(got, tk.ID)
		}
	}
	if want := []domain.TaskID{"match"}; !slices.Equal(got, want) {
		t.Errorf("matched %v, want %v", got, want)
	}
}

func TestCompareByDeadline(t *testing.T) {
	tasks := []*domain.Task{
		{ID: "none", Seq: 1},
		{ID: "late", Deadline: hoursAfter(5), Seq: 2},
		{ID: "early-second", Deadline: hoursAfter(1), Seq: 4},
		{ID: "early-first", Deadline: hoursAfter(1), Seq: 3},
	}

	slices.SortFunc(tasks, domain.CompareByDeadline)

	got := make([]domain.TaskID, len(tasks))
	for i, tk := range tasks {
		got[i] = tk.ID
	}
	if want := []domain.TaskID{"early-first", "early-second", "late", "none"}; !slices.Equal(got, want) {
		t.Errorf("sorted = %v, want %v", got, want)
	}
}
