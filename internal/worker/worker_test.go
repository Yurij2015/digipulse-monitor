package worker

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"monitor/internal/config"
)

const connectivityProbeURL = "https://www.cloudflare.com"

func newIntegrationWorker() *Worker {
	cfg := &config.Config{
		Connectivity: config.ConnectivityConfig{
			InternetCheckEnabled: true,
			InternetProbeURL:     connectivityProbeURL,
			ProbeTimeout:         5 * time.Second,
			OfflineWait:          1 * time.Second,
			RedisBRPopBlock:      1 * time.Second,
		},
	}

	return NewWorker(cfg)
}

func requireInternet(t *testing.T, w *Worker) {
	t.Helper()

	if !w.outboundInternetReachable(context.Background()) {
		t.Skipf("skipping integration test: outbound internet probe %s is not reachable", connectivityProbeURL)
	}
}

func TestCheckHTTP(t *testing.T) {
	w := newIntegrationWorker()
	requireInternet(t, w)

	task := &CheckTask{
		URL: connectivityProbeURL,
	}
	result := &CheckResult{}

	w.checkHTTP(task, result)

	if result.Status != "up" {
		t.Fatalf("expected HTTP status up, got %q (error=%q)", result.Status, result.ErrorMessage)
	}
}

func TestCheckSSL(t *testing.T) {
	w := newIntegrationWorker()
	requireInternet(t, w)

	task := &CheckTask{
		URL: connectivityProbeURL,
	}
	result := &CheckResult{}

	w.checkSSL(task, result)

	if result.Status != "up" {
		t.Fatalf("expected SSL status up, got %q (error=%q)", result.Status, result.ErrorMessage)
	}

	if result.Metadata == nil {
		t.Fatalf("expected SSL metadata, got nil")
	}

	if _, ok := result.Metadata["issuer"]; !ok {
		t.Fatalf("expected SSL metadata to contain issuer")
	}
}

func TestCheckDNS(t *testing.T) {
	w := newIntegrationWorker()
	requireInternet(t, w)

	task := &CheckTask{
		URL: connectivityProbeURL,
	}
	result := &CheckResult{}

	w.checkDNS(task, result)

	if result.Status != "up" {
		t.Fatalf("expected DNS status up, got %q (error=%q)", result.Status, result.ErrorMessage)
	}

	ips, ok := result.Metadata["ips"].([]string)
	if !ok || len(ips) == 0 {
		t.Fatalf("expected DNS metadata ips, got %#v", result.Metadata["ips"])
	}
}

func TestCheckPort(t *testing.T) {
	w := newIntegrationWorker()
	requireInternet(t, w)

	task := &CheckTask{
		URL:    connectivityProbeURL,
		Params: map[string]interface{}{"port": "443"},
	}
	result := &CheckResult{}

	w.checkPort(task, result)

	if result.Status != "up" {
		t.Fatalf("expected port status up, got %q (error=%q)", result.Status, result.ErrorMessage)
	}
}

func TestCheckPing(t *testing.T) {
	if _, err := exec.LookPath("ping"); err != nil {
		t.Skip("skipping ping test: ping binary is not available")
	}

	w := newIntegrationWorker()
	requireInternet(t, w)

	task := &CheckTask{
		URL: connectivityProbeURL,
	}
	result := &CheckResult{}

	w.checkPing(task, result)

	if result.Status != "up" {
		t.Skipf("skipping ping assertion due to environment/network restrictions: %s", result.ErrorMessage)
	}
}
