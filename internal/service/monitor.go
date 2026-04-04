package service

import (
	"time"
)

// MonitorService provides monitoring functionality
type MonitorService struct {
	startTime time.Time
	version   string
}

// NewMonitorService creates a new monitor service
func NewMonitorService(version string) *MonitorService {
	return &MonitorService{
		startTime: time.Now(),
		version:   version,
	}
}

// GetUptime returns the service uptime
func (s *MonitorService) GetUptime() time.Duration {
	return time.Since(s.startTime)
}

// GetVersion returns the service version
func (s *MonitorService) GetVersion() string {
	return s.version
}

// GetStartTime returns the service start time
func (s *MonitorService) GetStartTime() time.Time {
	return s.startTime
}

// ServiceStatus represents the current service status
type ServiceStatus struct {
	Status    string        `json:"status"`
	Uptime    time.Duration `json:"uptime"`
	Version   string        `json:"version"`
	StartTime time.Time     `json:"start_time"`
}

// GetStatus returns the current service status
func (s *MonitorService) GetStatus() ServiceStatus {
	return ServiceStatus{
		Status:    "running",
		Uptime:    s.GetUptime(),
		Version:   s.version,
		StartTime: s.startTime,
	}
}