package worker

import (
	"context"
	"errors"
	"testing"
)

type mockPingRunner struct {
	output []byte
	err    error
	host   string
}

func (m *mockPingRunner) Run(_ context.Context, host string) ([]byte, error) {
	m.host = host

	return m.output, m.err
}

func TestApplyPingOutput_summaryLine(t *testing.T) {
	result := &CheckResult{}
	output := []byte(`PING example.com (93.184.216.34) 56(84) bytes of data.
--- example.com ping statistics ---
4 packets transmitted, 4 received, 0% packet loss, time 3003ms
rtt min/avg/max/mdev = 10.1/12.5/15.0/1.2 ms`)

	applyPingOutput(result, output, nil)

	if result.Status != "up" {
		t.Fatalf("expected status up, got %q", result.Status)
	}
	if result.ResponseTimeMS == nil || *result.ResponseTimeMS != 12 {
		t.Fatalf("expected avg RTT 12 ms, got %v", result.ResponseTimeMS)
	}
}

func TestApplyPingOutput_fallbackTimeLines(t *testing.T) {
	result := &CheckResult{}
	output := []byte(`64 bytes from 93.184.216.34: icmp_seq=1 ttl=56 time=10.2 ms
64 bytes from 93.184.216.34: icmp_seq=2 ttl=56 time=14.8 ms`)

	applyPingOutput(result, output, nil)

	if result.Status != "up" {
		t.Fatalf("expected status up, got %q", result.Status)
	}
	if result.ResponseTimeMS == nil || *result.ResponseTimeMS != 12 {
		t.Fatalf("expected averaged RTT 12 ms, got %v", result.ResponseTimeMS)
	}
}

func TestApplyPingOutput_commandError(t *testing.T) {
	result := &CheckResult{}
	applyPingOutput(result, nil, errors.New("exit status 1"))

	if result.Status != "down" {
		t.Fatalf("expected status down, got %q", result.Status)
	}
	if result.ErrorMessage == "" {
		t.Fatal("expected error message")
	}
}

func TestApplyPingOutput_normalizesZeroRTT(t *testing.T) {
	result := &CheckResult{}
	applyPingOutput(result, []byte("rtt min/avg/max/mdev = 0.0/0.0/0.0/0.0 ms"), nil)

	if result.Status != "up" {
		t.Fatalf("expected status up, got %q", result.Status)
	}
	if result.ResponseTimeMS == nil || *result.ResponseTimeMS != 1 {
		t.Fatalf("expected RTT normalized to 1 ms, got %v", result.ResponseTimeMS)
	}
}

func TestCheckPing_successWithMock(t *testing.T) {
	mock := &mockPingRunner{
		output: []byte("rtt min/avg/max/mdev = 1.0/8.0/9.0/0.5 ms"),
	}
	w := &Worker{pingRunner: mock}

	// Reserved TLD: no DNS lookup side effects in unit tests.
	task := &CheckTask{URL: "https://target.invalid"}
	result := &CheckResult{}

	w.checkPing(task, result)

	if mock.host != "target.invalid" {
		t.Fatalf("expected ping host target.invalid, got %q", mock.host)
	}
	if result.Status != "up" {
		t.Fatalf("expected status up, got %q (error=%q)", result.Status, result.ErrorMessage)
	}
	if result.ResponseTimeMS == nil || *result.ResponseTimeMS != 8 {
		t.Fatalf("expected RTT 8 ms, got %v", result.ResponseTimeMS)
	}
}

func TestCheckPing_failureWithMock(t *testing.T) {
	mock := &mockPingRunner{err: errors.New("exit status 1")}
	w := &Worker{pingRunner: mock}

	task := &CheckTask{URL: "https://target.invalid"}
	result := &CheckResult{}

	w.checkPing(task, result)

	if mock.host != "target.invalid" {
		t.Fatalf("expected ping host target.invalid, got %q", mock.host)
	}
	if result.Status != "down" {
		t.Fatalf("expected status down, got %q", result.Status)
	}
	if result.ErrorMessage == "" {
		t.Fatal("expected error message")
	}
}
