package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptrace"
	"strconv"
	"strings"
	"time"

	"monitor/internal/config"
	"monitor/internal/service"

	"github.com/redis/go-redis/v9"
)

const slowThresholdMS = 3000

type CheckTask struct {
	ID              string      `json:"id"`
	ConfigurationID uint        `json:"configuration_id"`
	SiteID          uint        `json:"site_id"`
	URL             string      `json:"url"`
	Type            string      `json:"type"`
	Params          interface{} `json:"params"`
	UpdateInterval  int         `json:"update_interval"`
	ScheduledAt     string      `json:"scheduled_at"`
}

func (t *CheckTask) GetParamsMap() map[string]interface{} {
	if m, ok := t.Params.(map[string]interface{}); ok {
		return m
	}
	return make(map[string]interface{})
}

type CheckResult struct {
	ConfigurationID uint                   `json:"configuration_id"`
	Status          string                 `json:"status"`
	ResponseTimeMS  *int64                 `json:"response_time_ms,omitempty"`
	ErrorMessage    string                 `json:"error_message,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

type Worker struct {
	cfg        *config.Config
	redis      *redis.Client
	geo        *service.GeoIPService
	ssl        *service.SSLService
	pingRunner PingRunner
}

func NewWorker(cfg *config.Config) *Worker {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	return &Worker{
		cfg:        cfg,
		redis:      rdb,
		geo:        service.NewGeoIPService(),
		ssl:        service.NewSSLService(),
		pingRunner: ExecPingRunner{},
	}
}

func (w *Worker) Start(ctx context.Context) {
	log.Printf("Starting Redis worker (Queue mode) on key: %s", w.cfg.Redis.ChannelName)
	if w.cfg.Connectivity.InternetCheckEnabled {
		log.Printf("Outbound internet check enabled (probe: %s)", w.cfg.Connectivity.InternetProbeURL)
	}

	for {
		select {
		case <-ctx.Done():
			log.Println("Worker shutting down...")
			return
		default:
			// Heartbeat = worker process is alive and Redis is reachable (not outbound internet).
			w.sendHeartbeat(ctx)

			if w.cfg.Connectivity.InternetCheckEnabled && !w.outboundInternetReachable(ctx) {
				log.Printf("No outbound internet (probe %s); not dequeuing tasks. Retrying in %v...",
					w.cfg.Connectivity.InternetProbeURL, w.cfg.Connectivity.OfflineWait)
				if !w.sleepOrDone(ctx, w.cfg.Connectivity.OfflineWait) {
					return
				}

				continue
			}

			brPopBlock := w.cfg.Connectivity.RedisBRPopBlock
			if !w.cfg.Connectivity.InternetCheckEnabled {
				brPopBlock = 0
			}

			// BRPOP returns [key, value]. Non-zero block re-runs the internet probe when the queue is idle.
			res, err := w.redis.BRPop(ctx, brPopBlock, w.cfg.Redis.ChannelName).Result()
			if errors.Is(err, redis.Nil) {
				continue
			}

			if err != nil {
				if err != ctx.Err() {
					log.Printf("Error popping task: %v", err)
				}
				continue
			}

			if len(res) < 2 {
				continue
			}

			var task CheckTask
			if err := json.Unmarshal([]byte(res[1]), &task); err != nil {
				log.Printf("Error unmarshaling task: %v", err)
				continue
			}

			go w.processTask(task)
		}
	}
}

func (w *Worker) outboundInternetReachable(ctx context.Context) bool {
	if !w.cfg.Connectivity.InternetCheckEnabled {
		return true
	}

	probeCtx, cancel := context.WithTimeout(ctx, w.cfg.Connectivity.ProbeTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, w.cfg.Connectivity.InternetProbeURL, nil)
	if err != nil {
		return false
	}

	req.Header.Set("User-Agent", "DigiPulse-Monitor/Connectivity-Probe")

	client := &http.Client{Timeout: w.cfg.Connectivity.ProbeTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}

	defer func(Body io.ReadCloser) {
		_, _ = io.Copy(io.Discard, Body)
		err := Body.Close()
		if err != nil {
			log.Printf("Error closing connectivity probe body: %v", err)
		}
	}(resp.Body)

	return resp.StatusCode < http.StatusInternalServerError
}

func (w *Worker) sleepOrDone(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}

func (w *Worker) processTask(task CheckTask) {
	if task.ScheduledAt != "" && task.UpdateInterval > 0 {
		scheduledTime, err := time.Parse(time.RFC3339, task.ScheduledAt)
		if err == nil {
			maxAge := time.Duration((task.UpdateInterval*2)-1) * time.Second
			if time.Since(scheduledTime) > maxAge {
				log.Printf("Skipping stale task [%s] for %s (Scheduled: %s, Max Age: %v)",
					task.ID, task.URL, task.ScheduledAt, maxAge)
				return
			}
		}
	}

	log.Printf("Processing [%s] check for Site: %s", task.Type, task.URL)

	var result CheckResult
	result.ConfigurationID = task.ConfigurationID

	switch task.Type {
	case "http":
		w.checkHTTP(&task, &result)
	case "ssl":
		w.checkSSL(&task, &result)
	case "dns":
		w.checkDNS(&task, &result)
	case "port":
		w.checkPort(&task, &result)
	case "ping":
		w.checkPing(&task, &result)
	default:
		w.checkHTTP(&task, &result) // Default to HTTP
	}

	// If the check was successful but took less than 1ms (e.g. local loopback),
	// record it as 1ms instead of 0ms so it doesn't look like an error.
	// SSL cache hits omit response_time_ms entirely.
	if result.Status == "up" && result.ResponseTimeMS != nil && *result.ResponseTimeMS <= 0 {
		one := int64(1)
		result.ResponseTimeMS = &one
	}

	w.reportResult(result)
}

func (w *Worker) checkHTTP(task *CheckTask, result *CheckResult) {
	start := time.Now()
	client := http.Client{Timeout: 10 * time.Second}

	var remoteIP string
	trace := &httptrace.ClientTrace{
		GotConn: func(connInfo httptrace.GotConnInfo) {
			remoteIP, _, _ = net.SplitHostPort(connInfo.Conn.RemoteAddr().String())
		},
	}

	req, err := http.NewRequest("GET", task.URL, nil)
	if err != nil {
		result.Status = "down"
		result.ErrorMessage = "Request creation failed: " + err.Error()
		return
	}

	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	// Set common headers to avoid being blocked as a bot
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; DigiPulse/1.0)")
	req.Header.Set("Accept", "*/*")

	resp, err := client.Do(req)
	ms := time.Since(start).Milliseconds()
	result.ResponseTimeMS = &ms

	w.enrichWithGeo(result, remoteIP)

	if err != nil {
		result.Status = "down"
		result.ErrorMessage = err.Error()
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("Error closing response body: %v", err)
		}
	}(resp.Body)

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 400:
		if *result.ResponseTimeMS >= slowThresholdMS {
			result.Status = "slow"
		} else {
			result.Status = "up"
		}
	default:
		result.Status = "down"
		result.ErrorMessage = fmt.Sprintf("HTTP Status: %d", resp.StatusCode)
	}
}

func (w *Worker) checkSSL(task *CheckTask, result *CheckResult) {
	info, probeMs, err := w.ssl.GetInfo(task.URL)
	result.ResponseTimeMS = probeMs

	if err != nil {
		result.Status = "down"
		result.ErrorMessage = "SSL Check Error: " + err.Error()
		return
	}

	result.Status = "up"
	md := map[string]interface{}{
		"issuer":         info.Issuer,
		"days_remaining": info.DaysRemaining,
		"expires_at":     info.ExpiresAt.Format(time.RFC3339),
	}
	if probeMs == nil {
		md["ssl_cert_from_cache"] = true
	}
	result.Metadata = md

	if ips, err := net.LookupIP(extractHost(task.URL)); err == nil && len(ips) > 0 {
		w.enrichWithGeo(result, ips[0].String())
	}
}

func (w *Worker) checkDNS(task *CheckTask, result *CheckResult) {
	start := time.Now()
	ips, err := net.LookupIP(extractHost(task.URL))
	ms := time.Since(start).Milliseconds()
	result.ResponseTimeMS = &ms

	if err != nil || len(ips) == 0 {
		result.Status = "down"
		result.ErrorMessage = "DNS Lookup failed"
		if err != nil {
			result.ErrorMessage += ": " + err.Error()
		}
		return
	}

	ipStrs := make([]string, len(ips))
	for i, ip := range ips {
		ipStrs[i] = ip.String()
	}

	result.Status = "up"
	result.Metadata = map[string]interface{}{
		"ips": ipStrs,
	}

	w.enrichWithGeo(result, ips[0].String())
}

func (w *Worker) checkPort(task *CheckTask, result *CheckResult) {
	params := task.GetParamsMap()
	portStr, ok := params["port"].(string)
	if !ok {
		// Try float64 if it came from JSON as number
		if portNum, ok := params["port"].(float64); ok {
			portStr = strconv.Itoa(int(portNum))
		} else {
			portStr = "443" // Default
		}
	}

	host := extractHost(task.URL)

	start := time.Now()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, portStr), 5*time.Second)
	ms := time.Since(start).Milliseconds()
	result.ResponseTimeMS = &ms

	if err != nil {
		result.Status = "down"
		result.ErrorMessage = "Port not reachable: " + err.Error()
		return
	}
	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			log.Printf("Error closing port connection: %v", err)
		}
	}(conn)

	result.Status = "up"

	if remoteIP, _, err := net.SplitHostPort(conn.RemoteAddr().String()); err == nil {
		w.enrichWithGeo(result, remoteIP)
	}
}

func (w *Worker) checkPing(task *CheckTask, result *CheckResult) {
	host := extractHost(task.URL)

	if ips, err := net.LookupIP(host); err == nil && len(ips) > 0 {
		w.enrichWithGeo(result, ips[0].String())
	}

	runner := w.pingRunner
	if runner == nil {
		runner = ExecPingRunner{}
	}

	output, err := runner.Run(context.Background(), host)
	applyPingOutput(result, output, err)
}

func (w *Worker) enrichWithGeo(result *CheckResult, ip string) {
	if ip == "" {
		return
	}
	if result.Metadata == nil {
		result.Metadata = make(map[string]interface{})
	}
	result.Metadata["ip"] = ip
	if w.geo == nil {
		return
	}
	if geo, err := w.geo.GetInfo(ip); err == nil {
		result.Metadata["country"] = geo.Country
		result.Metadata["country_code"] = geo.CountryCode
		result.Metadata["city"] = geo.City
		result.Metadata["isp"] = geo.ISP
		result.Metadata["org"] = geo.Org
	}
}

func extractHost(rawURL string) string {
	if idx := strings.Index(rawURL, "://"); idx != -1 {
		rawURL = rawURL[idx+3:]
	}
	if idx := strings.Index(rawURL, "/"); idx != -1 {
		rawURL = rawURL[:idx]
	}
	return rawURL
}

func (w *Worker) reportResult(result CheckResult) {
	payload, err := json.Marshal(result)
	if err != nil {
		log.Printf("Error marshaling result: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = w.redis.LPush(ctx, w.cfg.Redis.ResultsChannel, payload).Err()
	if err != nil {
		log.Printf("Error reporting result to Redis queue [%s]: %v", w.cfg.Redis.ResultsChannel, err)
		return
	}

	log.Printf("Successfully reported result to Redis queue [%s] for Config ID: %d (Status: %s)", w.cfg.Redis.ResultsChannel, result.ConfigurationID, result.Status)
}

func (w *Worker) sendHeartbeat(ctx context.Context) {
	hbCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := w.redis.Set(hbCtx, w.cfg.Redis.HeartbeatKey, time.Now().Unix(), 10*time.Minute).Err(); err != nil {
		log.Printf("Failed to send heartbeat: %v", err)
	}
}
