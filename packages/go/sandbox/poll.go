package sandbox

import (
	"context"
	"time"
)

// PollOption customizes SDK operations that poll server state until a terminal
// condition is reached, such as waiting for a sandbox or template build to be ready.
type PollOption func(*pollOpts)

type pollOpts struct {
	interval     time.Duration
	maxInterval  time.Duration
	backoff      float64
	onPoll       func(attempt int)
	onBuildLogs  func([]BuildLogEntry)
}

func defaultPollOpts(defaultInterval time.Duration) *pollOpts {
	return &pollOpts{
		interval:    defaultInterval,
		maxInterval: 0,
		backoff:     1.0,
	}
}

// WithPollInterval sets the initial delay between poll attempts. Values less
// than or equal to zero are normalized to one second by the poll loop.
func WithPollInterval(d time.Duration) PollOption {
	return func(o *pollOpts) { o.interval = d }
}

// WithBackoff increases the poll interval after each attempt. multiplier values
// greater than 1 enable exponential backoff; maxInterval caps the delay when it
// is greater than zero.
func WithBackoff(multiplier float64, maxInterval time.Duration) PollOption {
	return func(o *pollOpts) {
		o.backoff = multiplier
		o.maxInterval = maxInterval
	}
}

// WithOnPoll registers a callback invoked before each poll attempt. The first
// attempt is numbered 1.
func WithOnPoll(fn func(attempt int)) PollOption {
	return func(o *pollOpts) { o.onPoll = fn }
}

// WithOnBuildLogs registers a callback invoked with newly observed build log
// entries on each poll tick of WaitForBuild. Only entries not seen on
// previous ticks are passed in, so the callback forms a stream. When nil or
// unset, WaitForBuild does not fetch logs. Mirrors E2B's onBuildLogs.
func WithOnBuildLogs(fn func([]BuildLogEntry)) PollOption {
	return func(o *pollOpts) { o.onBuildLogs = fn }
}

func pollLoop[T any](ctx context.Context, opts *pollOpts, pollFn func() (bool, T, error)) (T, error) {
	if opts.interval <= 0 {
		opts.interval = time.Second
	}

	interval := opts.interval
	var timer *time.Timer
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()

	attempt := 0
	for {
		attempt++
		if opts.onPoll != nil {
			opts.onPoll(attempt)
		}

		done, result, err := pollFn()
		if err != nil {
			return result, err
		}
		if done {
			return result, nil
		}

		if opts.backoff > 1.0 {
			interval = time.Duration(float64(interval) * opts.backoff)
			if opts.maxInterval > 0 && interval > opts.maxInterval {
				interval = opts.maxInterval
			}
		}

		if timer == nil {
			timer = time.NewTimer(interval)
		} else {
			timer.Reset(interval)
		}
		select {
		case <-ctx.Done():
			var zero T
			return zero, ctx.Err()
		case <-timer.C:
		}
	}
}
