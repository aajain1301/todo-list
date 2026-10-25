package clock_test

import (
	"testing"
	"time"

	"todolist/internal/clock"
)

var start = time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)

func TestSystem_Now(t *testing.T) {
	before := time.Now()
	got := clock.System{}.Now()
	if got.Before(before) || got.After(time.Now()) {
		t.Errorf("System.Now() = %v, not within the call window", got)
	}
}

func TestFake_Now(t *testing.T) {
	if got := clock.NewFake(start).Now(); !got.Equal(start) {
		t.Errorf("Now() = %v, want %v", got, start)
	}
}

func TestFake_Advance(t *testing.T) {
	c := clock.NewFake(start)
	c.Advance(time.Hour)
	if got, want := c.Now(), start.Add(time.Hour); !got.Equal(want) {
		t.Errorf("after Advance(1h): Now() = %v, want %v", got, want)
	}

	defer func() {
		if recover() == nil {
			t.Error("Advance with a negative duration should panic")
		}
	}()
	c.Advance(-time.Minute)
}
