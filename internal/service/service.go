package service

import (
	"fmt"
	"slices"
	"sync"
	"time"

	"todolist/internal/clock"
	"todolist/internal/domain"
)

type TaskRepository interface {
	Save(t *domain.Task)
	Get(id domain.TaskID) (*domain.Task, bool)
	Pending() []*domain.Task
	Closed() []*domain.Task
}

type ActivityRepository interface {
	Append(e domain.ActivityEvent)
	Range(p *domain.TimePeriod) []domain.ActivityEvent
}

type TaskService struct {
	mu       sync.RWMutex
	clock    clock.Clock
	tasks    TaskRepository
	activity ActivityRepository
	seq      uint64
}

func New(clk clock.Clock, tasks TaskRepository, activity ActivityRepository) *TaskService {
	return &TaskService{clock: clk, tasks: tasks, activity: activity}
}

func (s *TaskService) AddTask(task domain.Task) (domain.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.clock.Now()
	t := task.Clone()
	t.Status = domain.StatusPending
	t.CreatedAt = now
	t.UpdatedAt = now
	if t.VisibleFrom.IsZero() {
		t.VisibleFrom = now
	}
	if err := t.Validate(); err != nil {
		return domain.Task{}, err
	}

	s.seq++
	t.Seq = s.seq
	t.ID = domain.TaskID(fmt.Sprintf("T-%d", s.seq))
	s.tasks.Save(t)
	s.record(now, domain.EventAdded, t)
	return *t.Clone(), nil
}

func (s *TaskService) GetTask(id domain.TaskID) (domain.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, err := s.find(id)
	if err != nil {
		return domain.Task{}, err
	}
	return *t.Clone(), nil
}

func (s *TaskService) ModifyTask(task domain.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, err := s.findPending(task.ID)
	if err != nil {
		return err
	}
	t := current.Clone()
	t.Title = task.Title
	t.Description = task.Description
	t.Tags = slices.Clone(task.Tags)
	t.Deadline = task.Deadline
	t.VisibleFrom = task.VisibleFrom
	if err := t.Validate(); err != nil {
		return err
	}

	now := s.clock.Now()
	t.UpdatedAt = now
	s.tasks.Save(t)
	s.record(now, domain.EventModified, t)
	return nil
}

func (s *TaskService) RemoveTask(id domain.TaskID) error {
	return s.close(id, domain.StatusRemoved, domain.EventRemoved)
}

func (s *TaskService) CompleteTask(id domain.TaskID) error {
	return s.close(id, domain.StatusCompleted, domain.EventCompleted)
}

func (s *TaskService) ListTasks(filter domain.TaskFilter) ([]domain.Task, error) {
	if err := filter.Validate(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	source := s.tasks.Pending
	if filter.Scope == domain.ScopeCompleted {
		source = s.tasks.Closed
	}
	matches := filter.Matcher()

	var found []*domain.Task
	for _, t := range source() {
		if t.Status != domain.StatusRemoved && matches(t) {
			found = append(found, t)
		}
	}
	slices.SortFunc(found, domain.CompareByDeadline)

	out := make([]domain.Task, len(found))
	for i, t := range found {
		out[i] = *t.Clone()
	}
	return out, nil
}

func (s *TaskService) GetStatistics(period *domain.TimePeriod) (domain.Statistics, error) {
	if err := period.Validate(); err != nil {
		return domain.Statistics{}, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var stats domain.Statistics
	for _, e := range s.activity.Range(period) {
		switch e.Type {
		case domain.EventAdded:
			stats.Added++
		case domain.EventCompleted:
			stats.Completed++
		}
	}

	// Spilled-over is counted by deadline, so it needs closed tasks too.
	now := s.clock.Now()
	for _, tasks := range [][]*domain.Task{s.tasks.Pending(), s.tasks.Closed()} {
		for _, t := range tasks {
			if t.SpilledOver(now) && period.Contains(t.Deadline) {
				stats.SpilledOver++
			}
		}
	}
	return stats, nil
}

func (s *TaskService) GetActivityLog(period *domain.TimePeriod) ([]domain.ActivityEvent, error) {
	if err := period.Validate(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activity.Range(period), nil
}

func (s *TaskService) close(id domain.TaskID, status domain.Status, event domain.EventType) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, err := s.findPending(id)
	if err != nil {
		return err
	}
	now := s.clock.Now()
	t := current.Clone()
	t.Status = status
	t.UpdatedAt = now
	if status == domain.StatusCompleted {
		t.CompletedAt = now
	} else {
		t.RemovedAt = now
	}
	s.tasks.Save(t)
	s.record(now, event, t)
	return nil
}

func (s *TaskService) find(id domain.TaskID) (*domain.Task, error) {
	t, ok := s.tasks.Get(id)
	if !ok || t.Status == domain.StatusRemoved {
		return nil, fmt.Errorf("%w: %s", domain.ErrTaskNotFound, id)
	}
	return t, nil
}

func (s *TaskService) findPending(id domain.TaskID) (*domain.Task, error) {
	t, err := s.find(id)
	if err != nil {
		return nil, err
	}
	if t.Status == domain.StatusCompleted {
		return nil, fmt.Errorf("%w: %s", domain.ErrTaskClosed, id)
	}
	return t, nil
}

func (s *TaskService) record(at time.Time, typ domain.EventType, t *domain.Task) {
	s.activity.Append(domain.ActivityEvent{At: at, Type: typ, TaskID: t.ID, OwnerID: t.OwnerID, TaskTitle: t.Title})
}
