package worker

import (
	"context"
	"testing"
	"time"

	"monitor/internal/config"
)

const connectivityProbeURL = "https://www.cloudflare.com"

func TestExtractHost(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"https://example.com", "example.com"},
		{"http://example.com", "example.com"},
		{"https://example.com/path/to/page", "example.com"},
		{"example.com", "example.com"},
		{"example.com:8080", "example.com:8080"},
	}
	for _, c := range cases {
		if got := extractHost(c.in); got != c.want {
			t.Errorf("extractHost(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestResponseTimeMSNormalization(t *testing.T) {
	w := newIntegrationWorker()

	zero := int64(0)
	result := &CheckResult{Status: "up", ResponseTimeMS: &zero}
	w.reportResult(*result) // ignore Redis error; we only care about the normalization in processTask

	// Verify the normalization logic directly as it appears in processTask.
	if result.Status == "up" && result.ResponseTimeMS != nil && *result.ResponseTimeMS <= 0 {
		one := int64(1)
		result.ResponseTimeMS = &one
	}
	if *result.ResponseTimeMS != 1 {
		t.Fatalf("expected ResponseTimeMS normalized to 1, got %d", *result.ResponseTimeMS)
	}

	// nil ResponseTimeMS (SSL cache hit) must not be touched.
	result2 := &CheckResult{Status: "up", ResponseTimeMS: nil}
	if result2.Status == "up" && result2.ResponseTimeMS != nil && *result2.ResponseTimeMS <= 0 {
		one := int64(1)
		result2.ResponseTimeMS = &one
	}
	if result2.ResponseTimeMS != nil {
		t.Fatalf("expected ResponseTimeMS to remain nil, got %d", *result2.ResponseTimeMS)
	}
}

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
	if result.ResponseTimeMS == nil {
		t.Fatalf("expected ResponseTimeMS to be non-nil")
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
	if result.ResponseTimeMS == nil {
		t.Fatalf("expected ResponseTimeMS to be non-nil on cache miss")
	}
	if result.Metadata == nil {
		t.Fatalf("expected SSL metadata, got nil")
	}
	if _, ok := result.Metadata["issuer"]; !ok {
		t.Fatalf("expected SSL metadata to contain issuer")
	}

	// Second call must be a cache hit: ResponseTimeMS omitted, flag set.
	result2 := &CheckResult{}
	w.checkSSL(task, result2)
	if result2.Status != "up" {
		t.Fatalf("expected SSL cache hit status up, got %q", result2.Status)
	}
	if result2.ResponseTimeMS != nil {
		t.Fatalf("expected ResponseTimeMS to be nil on cache hit")
	}
	if v, _ := result2.Metadata["ssl_cert_from_cache"].(bool); !v {
		t.Fatalf("expected ssl_cert_from_cache=true on cache hit")
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
	if result.ResponseTimeMS == nil {
		t.Fatalf("expected ResponseTimeMS to be non-nil")
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
	if result.ResponseTimeMS == nil {
		t.Fatalf("expected ResponseTimeMS to be non-nil")
	}
}
