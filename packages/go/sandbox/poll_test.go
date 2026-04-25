package sandbox

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPollLoopCompletesImmediately(t *testing.T) {
	opts := defaultPollOpts(10 * time.Millisecond)
	calls := 0
	result, err := pollLoop(context.Background(), opts, func() (bool, int, error) {
		calls++
		return true, 42, nil
	})
	if err != nil {
		t.Fatalf("pollLoop err: %v", err)
	}
	if result != 42 {
		t.Errorf("result = %d, want 42", result)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestPollLoopError(t *testing.T) {
	opts := defaultPollOpts(time.Millisecond)
	boom := errors.New("boom")
	_, err := pollLoop(context.Background(), opts, func() (bool, int, error) {
		return false, 0, boom
	})
	if !errors.Is(err, boom) {
		t.Errorf("err = %v, want boom", err)
	}
}

func TestPollLoopContextCancel(t *testing.T) {
	opts := defaultPollOpts(50 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	_, err := pollLoop(ctx, opts, func() (bool, int, error) {
		return false, 0, nil
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want DeadlineExceeded", err)
	}
}

func TestPollLoopOnPollCallback(t *testing.T) {
	attempts := []int{}
	opts := defaultPollOpts(time.Millisecond)
	opts.onPoll = func(attempt int) { attempts = append(attempts, attempt) }
	stopAt := 3
	_, err := pollLoop(context.Background(), opts, func() (bool, int, error) {
		return len(attempts) >= stopAt, 0, nil
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(attempts) != 3 || attempts[0] != 1 || attempts[2] != 3 {
		t.Errorf("attempts = %v, want [1 2 3]", attempts)
	}
}

func TestPollLoopBackoffCaps(t *testing.T) {
	opts := defaultPollOpts(time.Millisecond)
	opts.backoff = 10.0
	opts.maxInterval = 5 * time.Millisecond
	// Complete after a few attempts; we only check this finishes quickly
	// because the backoff is capped.
	deadline := time.Now().Add(500 * time.Millisecond)
	n := 0
	_, err := pollLoop(context.Background(), opts, func() (bool, int, error) {
		n++
		return n >= 5, 0, nil
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if time.Now().After(deadline) {
		t.Error("expected backoff to be capped by maxInterval")
	}
}

func TestWithPollOptions(t *testing.T) {
	o := defaultPollOpts(time.Second)
	WithPollInterval(2 * time.Second)(o)
	if o.interval != 2*time.Second {
		t.Errorf("interval = %v", o.interval)
	}
	WithBackoff(1.5, 10*time.Second)(o)
	if o.backoff != 1.5 || o.maxInterval != 10*time.Second {
		t.Errorf("backoff = %v, maxInterval = %v", o.backoff, o.maxInterval)
	}
	called := false
	WithOnPoll(func(int) { called = true })(o)
	o.onPoll(1)
	if !called {
		t.Error("onPoll not invoked")
	}
	logsCalled := false
	WithOnBuildLogs(func([]BuildLogEntry) { logsCalled = true })(o)
	o.onBuildLogs(nil)
	if !logsCalled {
		t.Error("onBuildLogs not invoked")
	}
}
