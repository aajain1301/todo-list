package repository

import (
	"maps"
	"slices"
	"sort"

	"todolist/internal/domain"
)

type TaskRepository struct {
	pending map[domain.TaskID]*domain.Task
	closed  map[domain.TaskID]*domain.Task
}

func NewTaskRepository() *TaskRepository {
	return &TaskRepository{
		pending: map[domain.TaskID]*domain.Task{},
		closed:  map[domain.TaskID]*domain.Task{},
	}
}

func (r *TaskRepository) Save(t *domain.Task) {
	if t.Status == domain.StatusPending {
		r.pending[t.ID] = t
		return
	}
	delete(r.pending, t.ID)
	r.closed[t.ID] = t
}

func (r *TaskRepository) Get(id domain.TaskID) (*domain.Task, bool) {
	if t, ok := r.pending[id]; ok {
		return t, true
	}
	t, ok := r.closed[id]
	return t, ok
}

func (r *TaskRepository) Pending() []*domain.Task {
	return slices.Collect(maps.Values(r.pending))
}

func (r *TaskRepository) Closed() []*domain.Task {
	return slices.Collect(maps.Values(r.closed))
}

type ActivityRepository struct {
	events []domain.ActivityEvent
}

func (r *ActivityRepository) Append(e domain.ActivityEvent) {
	r.events = append(r.events, e)
}

func (r *ActivityRepository) Range(p *domain.TimePeriod) []domain.ActivityEvent {
	if p == nil {
		return slices.Clone(r.events)
	}
	lo := sort.Search(len(r.events), func(i int) bool { return !r.events[i].At.Before(p.Start) })
	hi := sort.Search(len(r.events), func(i int) bool { return !r.events[i].At.Before(p.End) })
	return slices.Clone(r.events[lo:hi])
}
