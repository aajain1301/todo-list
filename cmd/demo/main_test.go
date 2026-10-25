package main

import (
	"strings"
	"testing"

	"todolist/internal/clock"
	"todolist/internal/domain"
)

func TestNewService(t *testing.T) {
	svc := newService(clock.NewFake(start))

	added, err := svc.AddTask(domain.Task{OwnerID: "alice", Title: "Write report"})
	if err != nil {
		t.Fatalf("AddTask: %v", err)
	}
	if got, err := svc.GetTask(added.ID); err != nil || got.Title != "Write report" {
		t.Errorf("GetTask = %+v, %v; want the added task", got, err)
	}
}

func TestRun(t *testing.T) {
	var out strings.Builder
	run(&out, clock.NewFake(start))

	for _, want := range []string{
		"completed: T-1 Write final report",
		"stats: {Added:2 Completed:1 SpilledOver:1}",
		"MODIFIED", "COMPLETED", "REMOVED",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output is missing %q:\n%s", want, out.String())
		}
	}
}
