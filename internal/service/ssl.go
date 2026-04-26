package service

import (
	"crypto/tls"
	"net"
	"strings"
	"sync"
	"time"
)

type SSLInfo struct {
	Issuer        string    `json:"issuer"`
	DaysRemaining int       `json:"days_remaining"`
	ExpiresAt     time.Time `json:"expires_at"`
	LastChecked   time.Time `json:"last_checked"`
}

type SSLService struct {
	cache map[string]SSLInfo
	mu    sync.RWMutex
}

func NewSSLService() *SSLService {
	return &SSLService{
		cache: make(map[string]SSLInfo),
	}
}

func (s *SSLService) GetInfo(url string) (*SSLInfo, error) {
	s.mu.RLock()
	cached, ok := s.cache[url]
	s.mu.RUnlock()

	// Cache for 24 hours
	if ok && time.Since(cached.LastChecked) < 24*time.Hour {
		return &cached, nil
	}

	host := url
	if strings.HasPrefix(host, "http://") {
		host = strings.Replace(host, "http://", "", 1)
	} else if strings.HasPrefix(host, "https://") {
		host = strings.Replace(host, "https://", "", 1)
	}

	if idx := strings.Index(host, "/"); idx != -1 {
		host = host[:idx]
	}

	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 5 * time.Second}, "tcp", host+":443", nil)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	cert := conn.ConnectionState().PeerCertificates[0]
	daysRemaining := int(time.Until(cert.NotAfter).Hours() / 24)

	info := SSLInfo{
		Issuer:        cert.Issuer.CommonName,
		DaysRemaining: daysRemaining,
		ExpiresAt:     cert.NotAfter,
		LastChecked:   time.Now(),
	}

	s.mu.Lock()
	s.cache[url] = info
	s.mu.Unlock()

	return &info, nil
}
