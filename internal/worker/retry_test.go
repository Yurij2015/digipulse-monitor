package worker

import (
	"context"
	"errors"
	"testing"

	"monitor/internal/config"
)

// sequencePingRunner returns the queued responses in order, repeating the last one.
type sequencePingRunner struct {
	errs  []error
	calls int
}

func (s *sequencePingRunner) Run(_ context.Context, _ string) ([]byte, error) {
	idx := min(s.calls, len(s.errs)-1)
	s.calls++
	if err := s.errs[idx]; err != nil {
		return nil, err
	}

	return []byte("rtt min/avg/max/mdev = 1.0/8.0/9.0/0.5 ms"), nil
}

func newRetryWorker(runner PingRunner, maxRetries int) *Worker {
	return &Worker{
		cfg:        &config.Config{Retry: config.RetryConfig{MaxRetries: maxRetries}},
		pingRunner: runner,
	}
}

var pingTask = CheckTask{ConfigurationID: 42, Type: "ping", URL: "https://target.invalid"}

func TestRunCheckWithRetry_transientFailureRecovers(t *testing.T) {
	runner := &sequencePingRunner{errs: []error{errors.New("exit status 1"), nil}}
	w := newRetryWorker(runner, 2)

	result := w.runCheckWithRetry(pingTask, nil)

	if result.Status != "up" {
		t.Fatalf("expected up after retry, got %q (err=%q)", result.Status, result.ErrorMessage)
	}
	if result.ErrorMessage != "" {
		t.Fatalf("expected no error message from failed attempt, got %q", result.ErrorMessage)
	}
	if runner.calls != 2 {
		t.Fatalf("expected 2 attempts, got %d", runner.calls)
	}
	if result.ConfigurationID != 42 {
		t.Fatalf("expected configuration_id 42, got %d", result.ConfigurationID)
	}
}

func TestRunCheckWithRetry_persistentFailureReportsDown(t *testing.T) {
	runner := &sequencePingRunner{errs: []error{errors.New("exit status 1")}}
	w := newRetryWorker(runner, 2)

	result := w.runCheckWithRetry(pingTask, nil)

	if result.Status != "down" {
		t.Fatalf("expected down, got %q", result.Status)
	}
	if runner.calls != 3 {
		t.Fatalf("expected 3 attempts (1 + 2 retries), got %d", runner.calls)
	}
}

func TestRunCheckWithRetry_upDoesNotRetry(t *testing.T) {
	runner := &sequencePingRunner{errs: []error{nil}}
	w := newRetryWorker(runner, 2)

	if result := w.runCheckWithRetry(pingTask, nil); result.Status != "up" {
		t.Fatalf("expected up, got %q", result.Status)
	}
	if runner.calls != 1 {
		t.Fatalf("expected a single attempt, got %d", runner.calls)
	}
}

func TestRunCheckWithRetry_retriesDisabled(t *testing.T) {
	for _, maxRetries := range []int{0, -1} {
		runner := &sequencePingRunner{errs: []error{errors.New("exit status 1"), nil}}
		w := newRetryWorker(runner, maxRetries)

		if result := w.runCheckWithRetry(pingTask, nil); result.Status != "down" {
			t.Fatalf("RETRY_MAX=%d: expected down without retry, got %q", maxRetries, result.Status)
		}
		if runner.calls != 1 {
			t.Fatalf("RETRY_MAX=%d: expected a single attempt, got %d", maxRetries, runner.calls)
		}
	}
}
