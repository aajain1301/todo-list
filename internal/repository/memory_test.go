package repository_test

import (
	"slices"
	"testing"
	"time"

	"todolist/internal/domain"
	"todolist/internal/repository"
)

var start = time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)

func ids(tasks []*domain.Task) []domain.TaskID {
	out := make([]domain.TaskID, len(tasks))
	for i, t := range tasks {
		out[i] = t.ID
	}
	slices.Sort(out)
	return out
}

func seeded() *repository.TaskRepository {
	r := repository.NewTaskRepository()
	r.Save(&domain.Task{ID: "T-1", Status: domain.StatusPending})
	r.Save(&domain.Task{ID: "T-2", Status: domain.StatusCompleted})
	r.Save(&domain.Task{ID: "T-3", Status: domain.StatusRemoved})
	return r
}

func TestTaskRepository_Save(t *testing.T) {
	r := repository.NewTaskRepository()
	r.Save(&domain.Task{ID: "T-1", Status: domain.StatusPending})
	r.Save(&domain.Task{ID: "T-1", Status: domain.StatusCompleted})

	if got := r.Pending(); len(got) != 0 {
		t.Errorf("completed task still pending: %v", ids(got))
	}
	if got := ids(r.Closed()); !slices.Equal(got, []domain.TaskID{"T-1"}) {
		t.Errorf("Closed() = %v, want [T-1]", got)
	}
}

func TestTaskRepository_Get(t *testing.T) {
	r := seeded()
	for _, id := range []domain.TaskID{"T-1", "T-2"} {
		if got, ok := r.Get(id); !ok || got.ID != id {
			t.Errorf("Get(%s) = %v, %v; want the task", id, got, ok)
		}
	}
	if _, ok := r.Get("missing"); ok {
		t.Error("Get(missing) should report not found")
	}
}

func TestTaskRepository_Pending(t *testing.T) {
	if got := ids(seeded().Pending()); !slices.Equal(got, []domain.TaskID{"T-1"}) {
		t.Errorf("Pending() = %v, want [T-1]", got)
	}
}

func TestTaskRepository_Closed(t *testing.T) {
	if got := ids(seeded().Closed()); !slices.Equal(got, []domain.TaskID{"T-2", "T-3"}) {
		t.Errorf("Closed() = %v, want [T-2 T-3]", got)
	}
}

func TestActivityRepository_Append(t *testing.T) {
	var r repository.ActivityRepository
	r.Append(domain.ActivityEvent{TaskID: "T-1", At: start})
	r.Append(domain.ActivityEvent{TaskID: "T-2", At: start})
	if got := r.Range(nil); len(got) != 2 || got[0].TaskID != "T-1" || got[1].TaskID != "T-2" {
		t.Errorf("events = %v, want T-1 then T-2", got)
	}
}

func TestActivityRepository_Range(t *testing.T) {
	var r repository.ActivityRepository
	for h, id := range []domain.TaskID{"T-0", "T-1", "T-2", "T-3"} {
		r.Append(domain.ActivityEvent{TaskID: id, At: start.Add(time.Duration(h) * time.Hour)})
	}

	got := r.Range(&domain.TimePeriod{Start: start.Add(time.Hour), End: start.Add(3 * time.Hour)})

	var gotIDs []domain.TaskID
	for _, e := range got {
		gotIDs = append(gotIDs, e.TaskID)
	}
	if want := []domain.TaskID{"T-1", "T-2"}; !slices.Equal(gotIDs, want) {
		t.Errorf("Range = %v, want %v (start included, end excluded)", gotIDs, want)
	}
}
