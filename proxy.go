package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

// ProxyHandler manages HTTP proxying with authentication
type ProxyHandler struct {
	lb      LoadBalancer
	apiKey  string
	logChan chan<- RequestLog
}

// RequestLog records a single request's details
type RequestLog struct {
	Timestamp     time.Time
	LatencyMs     int64
	UpstreamName  string
	StatusCode    int
	RequestID     string
}

// NewProxyHandler creates a new proxy handler
func NewProxyHandler(lb LoadBalancer, apiKey string, logChan chan RequestLog) *ProxyHandler {
	return &ProxyHandler{
		lb:      lb,
		apiKey:  apiKey,
		logChan: logChan,
	}
}

// ServeHTTP handles incoming HTTP requests
func (h *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only handle POST /v1/messages
	if r.Method != http.MethodPost || r.URL.Path != "/v1/messages" {
		http.NotFound(w, r)
		return
	}

	// Auth check: extract token from x-api-key header OR Authorization: Bearer
	token := r.Header.Get("x-api-key")
	if token == "" {
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}

	if token != h.apiKey {
		h.writeError(w, http.StatusUnauthorized, "authentication_error", "Invalid API key", 0)
		return
	}

	// Request ID extraction: x-request-id header, fallback to RemoteAddr
	requestID := r.Header.Get("x-request-id")
	if requestID == "" {
		requestID = r.RemoteAddr
	}

	// Upstream selection
	upstream := h.lb.Select(requestID)
	if upstream == nil {
		h.writeError(w, http.StatusBadGateway, "upstream_error", "No upstream available", 1001)
		return
	}

	// Proxy request to upstream
	h.proxyRequest(w, r, upstream, requestID)
}

func (h *ProxyHandler) proxyRequest(w http.ResponseWriter, r *http.Request, upstream *Upstream, requestID string) {
	start := time.Now()

	// Create upstream request
	req, err := http.NewRequest(http.MethodPost, upstream.URL, r.Body)
	if err != nil {
		h.writeError(w, http.StatusBadGateway, "upstream_error", err.Error(), 1001)
		return
	}

	// Copy headers
	req.Header.Set("Host", r.Host)
	req.Header.Set("Content-Type", r.Header.Get("Content-Type"))
	req.Header.Set("x-request-id", requestID)

	// Set auth header based on upstream auth type
	if upstream.AuthType == "bearer" {
		req.Header.Set("Authorization", "Bearer "+upstream.APIKey)
	} else {
		req.Header.Set("x-api-key", upstream.APIKey)
	}

	// Copy any other headers we want to pass through
	for _, key := range []string{"Accept", "Accept-Encoding", "User-Agent"} {
		if val := r.Header.Get(key); val != "" {
			req.Header.Set(key, val)
		}
	}

	// Send request with timeout
	client := &http.Client{Timeout: upstream.Timeout}
	resp, err := client.Do(req)
	latencyMs := time.Since(start).Milliseconds()

	if err != nil {
		h.logToChan(requestID, latencyMs, upstream.Name, 0)
		h.writeError(w, http.StatusBadGateway, "upstream_error", err.Error(), 1001)
		return
	}
	defer resp.Body.Close()

	// Copy response to client
	h.logToChan(requestID, latencyMs, upstream.Name, resp.StatusCode)

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (h *ProxyHandler) writeError(w http.ResponseWriter, status int, errType, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	errResp := map[string]interface{}{
		"error": map[string]interface{}{
			"type":    errType,
			"message": message,
		},
	}
	if code > 0 {
		errResp["error"].(map[string]interface{})["code"] = code
	}
	json.NewEncoder(w).Encode(errResp)
}

func (h *ProxyHandler) logToChan(requestID string, latencyMs int64, upstreamName string, statusCode int) {
	if h.logChan != nil {
		h.logChan <- RequestLog{
			Timestamp:    time.Now(),
			LatencyMs:    latencyMs,
			UpstreamName: upstreamName,
			StatusCode:   statusCode,
			RequestID:    requestID,
		}
	}
}

// handleMessages is an alias for ServeHTTP for export compatibility
func (h *ProxyHandler) handleMessages(w http.ResponseWriter, r *http.Request) {
	h.ServeHTTP(w, r)
}
