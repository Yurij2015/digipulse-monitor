package service

import (
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

const sslCertCacheTTL = 24 * time.Hour

type SSLInfo struct {
	Issuer        string    `json:"issuer"`
	DaysRemaining int       `json:"days_remaining"`
	ExpiresAt     time.Time `json:"expires_at"`
}

type sslCachedEntry struct {
	info     SSLInfo
	cachedAt time.Time
}

type SSLService struct {
	mu    sync.RWMutex
	cache map[string]sslCachedEntry
}

func NewSSLService() *SSLService {
	return &SSLService{
		cache: make(map[string]sslCachedEntry),
	}
}

func (s *SSLService) GetInfo(url string) (*SSLInfo, *int64, error) {
	normalized := normalizeSSLURL(url)

	s.mu.RLock()
	entry, ok := s.cache[normalized]
	s.mu.RUnlock()

	if ok && time.Since(entry.cachedAt) < sslCertCacheTTL {
		info := entry.info
		return &info, nil, nil
	}

	start := time.Now()
	conn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: 5 * time.Second},
		"tcp",
		normalized+":443",
		&tls.Config{InsecureSkipVerify: true}, //nolint:gosec
	)
	probeMs := time.Since(start).Milliseconds()
	probePtr := &probeMs

	if err != nil {
		return nil, probePtr, err
	}
	defer conn.Close()

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return nil, probePtr, fmt.Errorf("no certificates presented by server")
	}

	cert := certs[0]
	daysRemaining := int(time.Until(cert.NotAfter).Hours() / 24)

	info := SSLInfo{
		Issuer:        cert.Issuer.CommonName,
		DaysRemaining: daysRemaining,
		ExpiresAt:     cert.NotAfter,
	}

	s.mu.Lock()
	s.cache[normalized] = sslCachedEntry{info: info, cachedAt: time.Now()}
	s.mu.Unlock()

	return &info, probePtr, nil
}

func normalizeSSLURL(url string) string {
	h := url
	if strings.HasPrefix(h, "http://") {
		h = strings.Replace(h, "http://", "", 1)
	} else if strings.HasPrefix(h, "https://") {
		h = strings.Replace(h, "https://", "", 1)
	}
	if idx := strings.Index(h, "/"); idx != -1 {
		h = h[:idx]
	}
	return h
}
