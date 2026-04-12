package worker

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"monitor/internal/config"

	"github.com/redis/go-redis/v9"
)

type CheckTask struct {
	ID              string      `json:"id"`
	ConfigurationID uint        `json:"configuration_id"`
	SiteID          uint        `json:"site_id"`
	URL             string      `json:"url"`
	Type            string      `json:"type"`
	Params          interface{} `json:"params"`
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
	}
}

func (w *Worker) Start(ctx context.Context) {
	log.Printf("Starting Redis worker on channel: %s", w.cfg.Redis.ChannelName)

	pubsub := w.redis.Subscribe(ctx, w.cfg.Redis.ChannelName)
	defer pubsub.Close()

	ch := pubsub.Channel()

	for {
		select {
		case <-ctx.Done():
			log.Println("Worker shutting down...")
			return
		case msg := <-ch:
			if msg == nil {
				continue
			}

			var task CheckTask
			if err := json.Unmarshal([]byte(msg.Payload), &task); err != nil {
				log.Printf("Error unmarshaling task: %v", err)
				continue
			}

			// Run check in a goroutine
			go w.processTask(task)
		}
	}
}

func (w *Worker) processTask(task CheckTask) {
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
	default:
		w.checkHTTP(&task, &result) // Default to HTTP
	}

	w.reportResult(result)
}

func (w *Worker) checkHTTP(task *CheckTask, result *CheckResult) {
	start := time.Now()
	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Head(task.URL)
	result.ResponseTimeMS = time.Since(start).Milliseconds()

	if err != nil {
		result.Status = "down"
		result.ErrorMessage = err.Error()
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		result.Status = "up"
	} else {
		result.Status = "down"
		result.ErrorMessage = fmt.Sprintf("HTTP Status: %d", resp.StatusCode)
	}
}

func (w *Worker) checkSSL(task *CheckTask, result *CheckResult) {
	start := time.Now()
	u := task.URL
	if strings.HasPrefix(u, "http://") {
		u = strings.Replace(u, "http://", "", 1)
	} else if strings.HasPrefix(u, "https://") {
		u = strings.Replace(u, "https://", "", 1)
	}

	// Strip path if any
	if idx := strings.Index(u, "/"); idx != -1 {
		u = u[:idx]
	}

	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 5 * time.Second}, "tcp", u+":443", nil)
	result.ResponseTimeMS = time.Since(start).Milliseconds()

	if err != nil {
		result.Status = "down"
		result.ErrorMessage = "SSL Dial Error: " + err.Error()
		return
	}
	defer conn.Close()

	cert := conn.ConnectionState().PeerCertificates[0]
	daysRemaining := int(time.Until(cert.NotAfter).Hours() / 24)

	result.Metadata = map[string]interface{}{
		"issuer":         cert.Issuer.CommonName,
		"days_remaining": daysRemaining,
		"expires_at":     cert.NotAfter.Format(time.RFC3339),
	}

	if daysRemaining < 0 {
		result.Status = "down"
		result.ErrorMessage = "Certificate expired"
	} else if daysRemaining < 7 {
		result.Status = "up" // Still reachable but warning could be added in metadata
	} else {
		result.Status = "up"
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
	defer conn.Close()

	result.Status = "up"
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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Backend reported error status: %d", resp.StatusCode)
	} else {
		log.Printf("Successfully reported result for Config ID: %d (Status: %s)", result.ConfigurationID, result.Status)
	}
}
