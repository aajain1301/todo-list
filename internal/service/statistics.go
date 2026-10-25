package service

import (
	"todolist/internal/clock"
	"todolist/internal/domain"
)

const (
	MetricAdded       = "added"
	MetricCompleted   = "completed"
	MetricSpilledOver = "spilled_over"
)

type Metric interface {
	Name() string
	Compute(period *domain.TimePeriod) int
}

type Statistics struct {
	metrics []Metric
}

func NewStatistics(metrics ...Metric) *Statistics {
	return &Statistics{metrics: metrics}
}

func (s *Statistics) GetAllStatistics(period *domain.TimePeriod) domain.Statistics {
	out := make(domain.Statistics, len(s.metrics))
	for _, m := range s.metrics {
		out[m.Name()] = m.Compute(period)
	}
	return out
}

type addedMetric struct{ activity ActivityRepository }

func (addedMetric) Name() string { return MetricAdded }

func (m addedMetric) Compute(period *domain.TimePeriod) int {
	return countEvents(m.activity, period, domain.EventAdded)
}

type completedMetric struct{ activity ActivityRepository }

func (completedMetric) Name() string { return MetricCompleted }

func (m completedMetric) Compute(period *domain.TimePeriod) int {
	return countEvents(m.activity, period, domain.EventCompleted)
}

// tasks whose deadline fell in the period and was missed (see Task.SpilledOver)
type spilledOverMetric struct {
	clock clock.Clock
	tasks TaskRepository
}

func (spilledOverMetric) Name() string { return MetricSpilledOver }

func (m spilledOverMetric) Compute(period *domain.TimePeriod) int {
	// Spilled-over is counted by deadline, so it needs closed tasks too.
	now := m.clock.Now()
	n := 0
	for _, tasks := range [][]*domain.Task{m.tasks.Pending(), m.tasks.Closed()} {
		for _, t := range tasks {
			if t.SpilledOver(now) && period.Contains(t.Deadline) {
				n++
			}
		}
	}
	return n
}

func countEvents(activity ActivityRepository, period *domain.TimePeriod, typ domain.EventType) int {
	n := 0
	for _, e := range activity.Range(period) {
		if e.Type == typ {
			n++
		}
	}
	return n
}
