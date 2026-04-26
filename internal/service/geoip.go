package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type GeoIPInfo struct {
	Country     string `json:"country"`
	CountryCode string `json:"countryCode"`
	City        string `json:"city"`
	ISP         string `json:"isp"`
	Org         string `json:"org"`
	Status      string `json:"status"`
}

type cachedInfo struct {
	info      *GeoIPInfo
	expiresAt time.Time
}

type GeoIPService struct {
	cache map[string]cachedInfo
	mu    sync.RWMutex
}

func NewGeoIPService() *GeoIPService {
	return &GeoIPService{
		cache: make(map[string]cachedInfo),
	}
}

func (s *GeoIPService) GetInfo(ip string) (*GeoIPInfo, error) {
	s.mu.RLock()
	cached, ok := s.cache[ip]
	s.mu.RUnlock()

	if ok && time.Now().Before(cached.expiresAt) {
		return cached.info, nil
	}

	// Fetch from ip-api.com
	url := fmt.Sprintf("http://ip-api.com/json/%s?fields=status,message,country,countryCode,city,isp,org", ip)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var info GeoIPInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}

	if info.Status != "success" {
		return nil, fmt.Errorf("GeoIP lookup failed for %s", ip)
	}

	// Cache result for 24 hours
	s.mu.Lock()
	s.cache[ip] = cachedInfo{
		info:      &info,
		expiresAt: time.Now().Add(24 * time.Hour),
	}
	s.mu.Unlock()

	return &info, nil
}
