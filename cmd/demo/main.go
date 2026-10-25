package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"todolist/internal/clock"
	"todolist/internal/domain"
	"todolist/internal/repository"
	"todolist/internal/service"
)

var start = time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)

func main() {
	run(os.Stdout, clock.NewFake(start))
}

func newService(clk clock.Clock) *service.TaskService {
	return service.New(clk, repository.NewTaskRepository(), &repository.ActivityRepository{})
}

func run(w io.Writer, clk *clock.Fake) {
	svc := newService(clk)

	report := must(svc.AddTask(domain.Task{OwnerID: "alice", Title: "Write report", Deadline: start.Add(time.Hour)}))
	gym := must(svc.AddTask(domain.Task{OwnerID: "bob", Title: "Gym"}))

	report = must(svc.GetTask(report.ID))
	report.Title = "Write final report"
	check(svc.ModifyTask(report))

	clk.Advance(2 * time.Hour) // past the report's deadline
	check(svc.CompleteTask(report.ID))
	check(svc.RemoveTask(gym.ID))

	for _, t := range must(svc.ListTasks(domain.TaskFilter{Scope: domain.ScopeCompleted})) {
		fmt.Fprintln(w, "completed:", t.ID, t.Title)
	}
	fmt.Fprintf(w, "stats: %+v\n", must(svc.GetStatistics(nil)))
	for _, e := range must(svc.GetActivityLog(nil)) {
		fmt.Fprintln(w, e)
	}
}

func must[T any](v T, err error) T {
	check(err)
	return v
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
