package proxy

import (
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"web-server/internal/config"
)

// ProxyHandler handles proxying requests to upstream servers
type ProxyHandler struct {
	upstreamServers []string
	currentServer   uint64
	config          *config.LocationConfig
	healthStatus    map[string]bool
	healthMutex     sync.RWMutex
	lastHealthCheck time.Time
}

// NewProxyHandler creates a new proxy handler
func NewProxyHandler(upstream *config.UpstreamConfig, config *config.LocationConfig) *ProxyHandler {
	handler := &ProxyHandler{
		upstreamServers: upstream.Servers,
		config:          config,
		healthStatus:    make(map[string]bool),
	}

	for _, server := range upstream.Servers {
		handler.healthStatus[server] = true
	}
	return handler
}

// Handle handles the proxy request
func (p *ProxyHandler) Handle(w http.ResponseWriter, r *http.Request) {
	if len(p.upstreamServers) == 0 {
		http.Error(w, "No upstream servers available", http.StatusServiceUnavailable)
		return
	}

	targetURL := p.selectUpstreamServer()
	remote, err := url.Parse(targetURL)
	if err != nil {
		http.Error(w, "Error parsing upstream server URL", http.StatusInternalServerError)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(remote)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		req.URL.Scheme = remote.Scheme
		req.URL.Host = remote.Host
		req.URL.Path = p.rewritePath(r.URL.Path, remote)

		clientIP := getClientIP(req)
		req.Header.Set("X-Forwarded-Host", req.Header.Get("Host"))
		req.Header.Set("X-Forwarded-Proto", getProto(req))
		req.Header.Set("X-Real-IP", clientIP)

		for header, value := range p.config.ProxySet {

			substitutedValue := strings.ReplaceAll(value, "$remote_addr", clientIP)
			substitutedValue = strings.ReplaceAll(substitutedValue, "$host", req.Header.Get("Host"))
			substitutedValue = strings.ReplaceAll(substitutedValue, "$scheme", getProto(req))
			req.Header.Set(header, substitutedValue)
		}

		if len(p.config.ProxyPassHeaders) > 0 {
			for _, header := range p.config.ProxyPassHeaders {
				if val := r.Header.Get(header); val != "" {
					req.Header.Set(header, val)
				}
			}
		}
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, "Error contacting upstream server", http.StatusBadGateway)
	}

	proxy.ServeHTTP(w, r)
}

func (p *ProxyHandler) selectUpstreamServer() string {
	// Perform health check
	if time.Since(p.lastHealthCheck) > 30*time.Second {
		p.performHealthChecks()
		p.lastHealthCheck = time.Now()
	}

	for i := 0; i < len(p.upstreamServers); i++ {
		server := p.upstreamServers[(p.currentServer+uint64(i))%uint64(len(p.upstreamServers))]
		p.healthMutex.RLock()
		isHealthy := p.healthStatus[server]
		p.healthMutex.RUnlock()

		if isHealthy {
			atomic.AddUint64(&p.currentServer, 1)
			return server
		}
	}

	server := p.upstreamServers[p.currentServer%uint64(len(p.upstreamServers))]
	atomic.AddUint64(&p.currentServer, 1)
	return server
}

func (p *ProxyHandler) performHealthChecks() {
	for _, server := range p.upstreamServers {
		go func(srv string) {
			client := &http.Client{
				Timeout: 2 * time.Second,
			}
			resp, err := client.Get(srv)
			if err != nil {
				p.healthMutex.Lock()
				p.healthStatus[srv] = false
				p.healthMutex.Unlock()
				return
			}
			defer resp.Body.Close()
			isHealthy := resp.StatusCode >= 200 && resp.StatusCode < 300
			p.healthMutex.Lock()
			p.healthStatus[srv] = isHealthy
			p.healthMutex.Unlock()
		}(server)
	}
}

// joinURLPath safely joins URL path segments without producing double slashes.
func joinURLPath(a, b string) string {
	a = strings.TrimRight(a, "/")
	b = strings.TrimLeft(b, "/")

	if a == "" && b == "" {
		return "/"
	}
	if a == "" {
		return "/" + b
	}
	if b == "" {
		return a
	}
	return a + "/" + b
}

// rewritePath implements nginx-like proxy_pass behavior based on the location
// path and upstream URL. If the location ends with "/", the location prefix
// is stripped from the incoming path and appended to the upstream URL path.
// If the location does NOT end with "/", nginx forwards the path unchanged.
func (p *ProxyHandler) rewritePath(originalPath string, upstream *url.URL) string {
	loc := p.config.Path

	// If location ends with "/", nginx strips the prefix.
	if strings.HasSuffix(loc, "/") {
		trimmed := strings.TrimPrefix(originalPath, loc)

		baseUp := upstream.Path
		if baseUp == "" {
			baseUp = "/"
		}

		return joinURLPath(baseUp, trimmed)
	}

	// No trailing slash → nginx does not rewrite the path.
	return originalPath
}

func getProto(r *http.Request) string {
	if r.Header.Get("X-Forwarded-Proto") != "" {
		return r.Header.Get("X-Forwarded-Proto")
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func getClientIP(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		ips := strings.Split(forwarded, ",")
		return strings.TrimSpace(ips[0])
	}

	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}

func (p *ProxyHandler) HealthCheck() map[string]bool {
	results := make(map[string]bool)

	for _, serverURL := range p.upstreamServers {
		_, err := url.Parse(serverURL)
		if err != nil {
			results[serverURL] = false
			continue
		}

		client := &http.Client{
			Timeout: 5 * time.Second,
		}

		resp, err := client.Get(serverURL)
		if err != nil {
			results[serverURL] = false
			continue
		}
		resp.Body.Close()

		results[serverURL] = resp.StatusCode >= 200 && resp.StatusCode < 300
	}

	return results
}
