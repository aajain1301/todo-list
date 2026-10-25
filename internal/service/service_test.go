package service_test

import (
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"todolist/internal/clock"
	"todolist/internal/domain"
	"todolist/internal/repository"
	"todolist/internal/service"
)

var start = time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)

func newService() (*service.TaskService, *clock.Fake) {
	clk := clock.NewFake(start)
	return service.New(clk, repository.NewTaskRepository(), &repository.ActivityRepository{}), clk
}

func mustAdd(t *testing.T, s *service.TaskService, task domain.Task) domain.Task {
	t.Helper()
	added, err := s.AddTask(task)
	if err != nil {
		t.Fatalf("AddTask(%q): %v", task.Title, err)
	}
	return added
}

func listIDs(t *testing.T, s *service.TaskService, f domain.TaskFilter) []domain.TaskID {
	t.Helper()
	tasks, err := s.ListTasks(f)
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	ids := make([]domain.TaskID, len(tasks))
	for i, task := range tasks {
		ids[i] = task.ID
	}
	return ids
}

func eventTypes(events []domain.ActivityEvent) []domain.EventType {
	types := make([]domain.EventType, len(events))
	for i, e := range events {
		types[i] = e.Type
	}
	return types
}

func TestAddTask(t *testing.T) {
	s, _ := newService()

	got := mustAdd(t, s, domain.Task{ID: "ignored", OwnerID: "alice", Title: "Write report", Tags: []string{"work"}})

	if got.ID != "T-1" || got.Title != "Write report" || !slices.Equal(got.Tags, []string{"work"}) {
		t.Errorf("AddTask = %+v, want ID T-1, title and tags kept", got)
	}
	if !got.VisibleFrom.Equal(start) || got.Status != domain.StatusPending {
		t.Errorf("VisibleFrom = %v, Status = %v; want %v, PENDING", got.VisibleFrom, got.Status, start)
	}
	log, _ := s.GetActivityLog(nil)
	if !slices.Equal(eventTypes(log), []domain.EventType{domain.EventAdded}) {
		t.Errorf("log = %v, want [ADDED]", eventTypes(log))
	}
	if _, err := s.AddTask(domain.Task{OwnerID: "alice"}); !errors.Is(err, domain.ErrInvalidTask) {
		t.Errorf("AddTask without title: err = %v, want ErrInvalidTask", err)
	}
}

func TestGetTask(t *testing.T) {
	s, _ := newService()
	task := mustAdd(t, s, domain.Task{OwnerID: "alice", Title: "Write report"})

	if got, err := s.GetTask(task.ID); err != nil || got.Title != "Write report" {
		t.Errorf("GetTask = %+v, %v; want the task", got, err)
	}
	if err := s.RemoveTask(task.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetTask(task.ID); !errors.Is(err, domain.ErrTaskNotFound) {
		t.Errorf("GetTask(removed): err = %v, want ErrTaskNotFound", err)
	}
}

func TestModifyTask(t *testing.T) {
	s, clk := newService()
	task := mustAdd(t, s, domain.Task{OwnerID: "alice", Title: "Write report", Tags: []string{"work"}})
	clk.Advance(time.Hour)

	task.Title = "Write final report"
	task.Deadline = start.Add(5 * time.Hour)
	if err := s.ModifyTask(task); err != nil {
		t.Fatalf("ModifyTask: %v", err)
	}

	got, _ := s.GetTask(task.ID)
	if got.Title != "Write final report" || !got.Deadline.Equal(task.Deadline) || !got.UpdatedAt.Equal(clk.Now()) {
		t.Errorf("after ModifyTask: %+v, want new title, deadline and UpdatedAt", got)
	}
	log, _ := s.GetActivityLog(nil)
	if want := []domain.EventType{domain.EventAdded, domain.EventModified}; !slices.Equal(eventTypes(log), want) {
		t.Errorf("log = %v, want %v", eventTypes(log), want)
	}
}

func TestRemoveTask(t *testing.T) {
	s, _ := newService()
	task := mustAdd(t, s, domain.Task{OwnerID: "alice", Title: "Write report"})

	if err := s.RemoveTask(task.ID); err != nil {
		t.Fatalf("RemoveTask: %v", err)
	}
	if ids := listIDs(t, s, domain.TaskFilter{Scope: domain.ScopePending}); len(ids) != 0 {
		t.Errorf("removed task still listed: %v", ids)
	}
	if err := s.RemoveTask(task.ID); !errors.Is(err, domain.ErrTaskNotFound) {
		t.Errorf("second RemoveTask: err = %v, want ErrTaskNotFound", err)
	}
}

func TestCompleteTask(t *testing.T) {
	s, _ := newService()
	task := mustAdd(t, s, domain.Task{OwnerID: "alice", Title: "Write report"})

	if err := s.CompleteTask(task.ID); err != nil {
		t.Fatalf("CompleteTask: %v", err)
	}
	if ids := listIDs(t, s, domain.TaskFilter{}); len(ids) != 0 {
		t.Errorf("completed task still on the TODO list: %v", ids)
	}
	if ids := listIDs(t, s, domain.TaskFilter{Scope: domain.ScopeCompleted}); !slices.Equal(ids, []domain.TaskID{task.ID}) {
		t.Errorf("completed scope = %v, want [%s]", ids, task.ID)
	}
	if err := s.CompleteTask(task.ID); !errors.Is(err, domain.ErrTaskClosed) {
		t.Errorf("second CompleteTask: err = %v, want ErrTaskClosed", err)
	}
}

func TestListTasks(t *testing.T) {
	s, clk := newService()
	later := mustAdd(t, s, domain.Task{OwnerID: "alice", Title: "Due later", Deadline: start.Add(5 * time.Hour)})
	sooner := mustAdd(t, s, domain.Task{OwnerID: "alice", Title: "Due sooner", Deadline: start.Add(2 * time.Hour)})
	future := mustAdd(t, s, domain.Task{OwnerID: "alice", Title: "File taxes", VisibleFrom: start.Add(24 * time.Hour)})

	if got, want := listIDs(t, s, domain.TaskFilter{}), []domain.TaskID{sooner.ID, later.ID}; !slices.Equal(got, want) {
		t.Errorf("active list = %v, want %v (future task hidden, sorted by deadline)", got, want)
	}

	clk.Advance(24 * time.Hour)
	if got := listIDs(t, s, domain.TaskFilter{}); !slices.Contains(got, future.ID) {
		t.Errorf("active list after VisibleFrom = %v, want it to include %s", got, future.ID)
	}

	if _, err := s.ListTasks(domain.TaskFilter{Scope: 42}); !errors.Is(err, domain.ErrInvalidFilter) {
		t.Errorf("unknown scope: err = %v, want ErrInvalidFilter", err)
	}
}

func TestGetStatistics(t *testing.T) {
	s, clk := newService()
	onTime := mustAdd(t, s, domain.Task{OwnerID: "alice", Title: "On time", Deadline: start.Add(time.Hour)})
	late := mustAdd(t, s, domain.Task{OwnerID: "alice", Title: "Late", Deadline: start.Add(time.Hour)})
	mustAdd(t, s, domain.Task{OwnerID: "alice", Title: "No deadline"})

	clk.Advance(30 * time.Minute)
	if err := s.CompleteTask(onTime.ID); err != nil {
		t.Fatal(err)
	}
	clk.Advance(2 * time.Hour)
	if err := s.CompleteTask(late.ID); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetStatistics(nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := (domain.Statistics{Added: 3, Completed: 2, SpilledOver: 1}); got != want {
		t.Errorf("all-time stats = %+v, want %+v", got, want)
	}

	quiet := &domain.TimePeriod{Start: start.Add(5 * time.Hour), End: start.Add(6 * time.Hour)}
	if got, _ := s.GetStatistics(quiet); got != (domain.Statistics{}) {
		t.Errorf("stats for an empty period = %+v, want zero", got)
	}
}

func TestGetActivityLog(t *testing.T) {
	s, clk := newService()
	mustAdd(t, s, domain.Task{OwnerID: "alice", Title: "First"})
	clk.Advance(time.Hour)
	second := mustAdd(t, s, domain.Task{OwnerID: "alice", Title: "Second"})
	clk.Advance(time.Hour)
	mustAdd(t, s, domain.Task{OwnerID: "alice", Title: "Third"})

	got, err := s.GetActivityLog(&domain.TimePeriod{Start: start.Add(time.Hour), End: start.Add(2 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].TaskID != second.ID {
		t.Errorf("log for [1h, 2h) = %v, want only the ADDED event of %s", got, second.ID)
	}

	inverted := &domain.TimePeriod{Start: start.Add(time.Hour), End: start}
	if _, err := s.GetActivityLog(inverted); !errors.Is(err, domain.ErrInvalidPeriod) {
		t.Errorf("inverted period: err = %v, want ErrInvalidPeriod", err)
	}
}

func TestConcurrentAccess(t *testing.T) {
	s, _ := newService()
	const writers, perWriter = 20, 50

	var wg sync.WaitGroup
	for w := range writers {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for i := range perWriter {
				owner := domain.UserID(fmt.Sprintf("user-%d", w))
				if _, err := s.AddTask(domain.Task{OwnerID: owner, Title: fmt.Sprintf("task %d", i)}); err != nil {
					t.Error(err)
				}
			}
		}()
		go func() {
			defer wg.Done()
			for range perWriter {
				if _, err := s.ListTasks(domain.TaskFilter{}); err != nil {
					t.Error(err)
				}
				if _, err := s.GetStatistics(nil); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()

	ids := listIDs(t, s, domain.TaskFilter{Scope: domain.ScopePending})
	if len(ids) != writers*perWriter {
		t.Fatalf("listed %d tasks, want %d", len(ids), writers*perWriter)
	}
	slices.Sort(ids)
	if len(slices.Compact(ids)) != writers*perWriter {
		t.Error("task IDs are not unique")
	}
	if log, _ := s.GetActivityLog(nil); len(log) != writers*perWriter {
		t.Errorf("activity log has %d events, want %d", len(log), writers*perWriter)
	}
}
