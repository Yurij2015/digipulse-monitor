package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"monitor/internal/config"

	"github.com/redis/go-redis/v9"
)

type CheckTask struct {
	ID              string                 `json:"id"`
	ConfigurationID uint                   `json:"configuration_id"`
	SiteID          uint                   `json:"site_id"`
	URL             string                 `json:"url"`
	Type            string                 `json:"type"`
	Params          map[string]interface{} `json:"params"`
	ScheduledAt     string                 `json:"scheduled_at"`
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
	log.Printf("Starting Redis worker on queue: %s", w.cfg.Redis.Key)

	for {
		select {
		case <-ctx.Done():
			log.Println("Worker shutting down...")
			return
		default:
			// BRPOP for blocking read
			result, err := w.redis.BRPop(ctx, 0, w.cfg.Redis.Key).Result()
			if err != nil {
				if err != context.Canceled {
					log.Printf("Error popping from Redis: %v", err)
				}
				continue
			}

			if len(result) < 2 {
				continue
			}

			payload := result[1]
			var task CheckTask
			if err := json.Unmarshal([]byte(payload), &task); err != nil {
				log.Printf("Error unmarshaling task: %v", err)
				continue
			}

			// Run check in a goroutine
			go w.processTask(task)
		}
	}
}

func (w *Worker) processTask(task CheckTask) {
	start := time.Now()
	log.Printf("Processing [%s] check for Site: %s", task.Type, task.URL)

	var result CheckResult
	result.ConfigurationID = task.ConfigurationID

	// Perform basic HTTP check for now
	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(task.URL)

	elapsed := time.Since(start).Milliseconds()
	result.ResponseTimeMS = elapsed

	if err != nil {
		result.Status = "down"
		result.ErrorMessage = err.Error()
	} else {
		defer resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 400 {
			result.Status = "up"
		} else {
			result.Status = "down"
			result.ErrorMessage = fmt.Sprintf("HTTP Status: %d", resp.StatusCode)
		}
	}

	w.reportResult(result)
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
