package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"monitor/internal/model"
)

// HealthHandler handles health check requests
type HealthHandler struct {
	version string
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(version string) *HealthHandler {
	return &HealthHandler{
		version: version,
	}
}

// HealthCheck godoc
// @Summary Health check endpoint
// @Description Returns the health status of the service
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} model.HealthResponse
// @Router /health [get]
func (h *HealthHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	response := model.HealthResponse{
		Status:    "ok",
		Timestamp: time.Now(),
		Version:   h.version,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		return
	}
}

// ServiceInfo godoc
// @Summary Get service information
// @Description Returns information about the service
// @Tags info
// @Accept json
// @Produce json
// @Success 200 {object} model.ServiceInfo
// @Router /info [get]
func (h *HealthHandler) ServiceInfo(w http.ResponseWriter, r *http.Request) {
	response := model.ServiceInfo{
		Name:        "monitor-service",
		Version:     h.version,
		Description: "Monitoring service for digi-pulse",
		Environment: "production",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		return
	}
}

// RootHandler godoc
// @Summary Root endpoint
// @Description Returns a welcome message
// @Tags root
// @Accept JSON
// @Produce JSON
// @Success 200 {object} map[string]string
// @Router / [get]
func (h *HealthHandler) RootHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{
		"message": "Monitor service is running",
		"version": h.version,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		return
	}
}
