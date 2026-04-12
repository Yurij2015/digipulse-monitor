package model

import "time"

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string    `json:"status" example:"ok"`
	Timestamp time.Time `json:"timestamp" example:"2024-01-01T00:00:00Z"`
	Version   string    `json:"version" example:"1.0.0"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error" example:"Something went wrong"`
	Code    int    `json:"code" example:"500"`
	Message string `json:"message" example:"Internal server error"`
}

// ServiceInfo represents service information
type ServiceInfo struct {
	Name        string `json:"name" example:"monitor-service"`
	Version     string `json:"version" example:"1.0.0"`
	Description string `json:"description" example:"Monitoring service for digi-pulse"`
	Environment string `json:"environment" example:"production"`
}
