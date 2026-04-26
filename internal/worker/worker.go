package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptrace"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"monitor/internal/config"
	"monitor/internal/service"

	"github.com/redis/go-redis/v9"
)

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
	ResponseTimeMS  int64                  `json:"response_time_ms"`
	ErrorMessage    string                 `json:"error_message,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

type Worker struct {
	cfg   *config.Config
	redis *redis.Client
	geo   *service.GeoIPService
	ssl   *service.SSLService
}

func NewWorker(cfg *config.Config) *Worker {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	return &Worker{
		cfg:   cfg,
		redis: rdb,
		geo:   service.NewGeoIPService(),
		ssl:   service.NewSSLService(),
	}
}

func (w *Worker) Start(ctx context.Context) {
	log.Printf("Starting Redis worker (Queue mode) on key: %s", w.cfg.Redis.ChannelName)

	for {
		select {
		case <-ctx.Done():
			log.Println("Worker shutting down...")
			return
		default:
			// BRPOP returns [key, value]
			res, err := w.redis.BRPop(ctx, 0, w.cfg.Redis.ChannelName).Result()
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

			// Run check in a goroutine
			go w.processTask(task)
		}
	}
}

func (w *Worker) processTask(task CheckTask) {
	// Expiration check: (update_interval * 2) - 1
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
	result.ResponseTimeMS = time.Since(start).Milliseconds()

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

	if resp.StatusCode < 500 {
		result.Status = "up"
	} else {
		result.Status = "down"
		result.ErrorMessage = fmt.Sprintf("HTTP Status: %d", resp.StatusCode)
	}
}

func (w *Worker) checkSSL(task *CheckTask, result *CheckResult) {
	start := time.Now()

	info, err := w.ssl.GetInfo(task.URL)
	result.ResponseTimeMS = time.Since(start).Milliseconds()

	if err != nil {
		result.Status = "down"
		result.ErrorMessage = "SSL Check Error: " + err.Error()
		return
	}

	result.Status = "up"
	result.Metadata = map[string]interface{}{
		"issuer":         info.Issuer,
		"days_remaining": info.DaysRemaining,
		"expires_at":     info.ExpiresAt.Format(time.RFC3339),
	}

	// Also try to get IP for Geo info
	host := task.URL
	if strings.HasPrefix(host, "http://") {
		host = strings.Replace(host, "http://", "", 1)
	} else if strings.HasPrefix(host, "https://") {
		host = strings.Replace(host, "https://", "", 1)
	}
	if idx := strings.Index(host, "/"); idx != -1 {
		host = host[:idx]
	}

	if ips, err := net.LookupIP(host); err == nil && len(ips) > 0 {
		w.enrichWithGeo(result, ips[0].String())
	}
}

func (w *Worker) checkDNS(task *CheckTask, result *CheckResult) {
	start := time.Now()
	host := task.URL
	if idx := strings.Index(host, "://"); idx != -1 {
		host = host[idx+3:]
	}
	if idx := strings.Index(host, "/"); idx != -1 {
		host = host[:idx]
	}

	ips, err := net.LookupIP(host)
	result.ResponseTimeMS = time.Since(start).Milliseconds()

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

	if len(ips) > 0 {
		w.enrichWithGeo(result, ips[0].String())
	}
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

	host := task.URL
	if idx := strings.Index(host, "://"); idx != -1 {
		host = host[idx+3:]
	}
	if idx := strings.Index(host, "/"); idx != -1 {
		host = host[:idx]
	}

	start := time.Now()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, portStr), 5*time.Second)
	result.ResponseTimeMS = time.Since(start).Milliseconds()

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
	host := task.URL
	if strings.HasPrefix(host, "http://") {
		host = strings.Replace(host, "http://", "", 1)
	} else if strings.HasPrefix(host, "https://") {
		host = strings.Replace(host, "https://", "", 1)
	}
	if idx := strings.Index(host, "/"); idx != -1 {
		host = host[:idx]
	}

	// Resolve IP for Geo info
	if ips, err := net.LookupIP(host); err == nil && len(ips) > 0 {
		w.enrichWithGeo(result, ips[0].String())
	}

	// Use system ping command
	cmd := exec.Command("ping", "-c", "4", "-W", "2", host)
	output, err := cmd.CombinedOutput()
	if err != nil {
		result.Status = "down"
		result.ErrorMessage = "Ping failed: " + err.Error()
		return
	}

	// Parse ping output for average RTT
	// Format can be 3 numbers (min/avg/max) or 4 (min/avg/max/stddev)
	re := regexp.MustCompile(`[\d.]+/([\d.]+)/[\d.]+`)
	matches := re.FindStringSubmatch(string(output))
	if len(matches) > 1 {
		avg, _ := strconv.ParseFloat(matches[1], 64)
		result.ResponseTimeMS = int64(avg)
	}

	result.Status = "up"
}

func (w *Worker) enrichWithGeo(result *CheckResult, ip string) {
	if ip == "" {
		return
	}
	if result.Metadata == nil {
		result.Metadata = make(map[string]interface{})
	}
	result.Metadata["ip"] = ip
	if geo, err := w.geo.GetInfo(ip); err == nil {
		result.Metadata["country"] = geo.Country
		result.Metadata["country_code"] = geo.CountryCode
		result.Metadata["city"] = geo.City
		result.Metadata["isp"] = geo.ISP
		result.Metadata["org"] = geo.Org
	}
}

func (w *Worker) reportResult(result CheckResult) {
	payload, err := json.Marshal(result)
	if err != nil {
		log.Printf("Error marshaling result: %v", err)
		return
	}

	url := fmt.Sprintf("%s/results", w.cfg.Backend.BaseURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		log.Printf("Error creating report request: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Monitor-Key", w.cfg.Backend.Key)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error reporting result: %v", err)
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("Error closing response body: %v", err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		log.Printf("Backend reported error status: %d", resp.StatusCode)
	} else {
		log.Printf("Successfully reported result for Config ID: %d (Status: %s)", result.ConfigurationID, result.Status)
	}
}
