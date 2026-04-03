package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	baseDelay  = 1 * time.Second
	maxRetries = 3
	maxDelay   = 4 * time.Second
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
	RetryAttempt  int // Current retry attempt (0=initial, 1+=retries)
	RetryCount    int // Total retries for this request
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

	// Proxy request with retry
	h.proxyWithRetry(w, r, requestID)
}

func (h *ProxyHandler) proxyRequest(w http.ResponseWriter, r *http.Request, upstream *Upstream, requestID string, retryAttempt, retryCount int) (error, int) {
	start := time.Now()

	// Create upstream request
	req, err := http.NewRequest(http.MethodPost, upstream.URL, r.Body)
	if err != nil {
		return err, 0
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
		h.logToChan(requestID, latencyMs, upstream.Name, 0, retryAttempt, retryCount)
		return err, 0
	}
	defer resp.Body.Close()

	// Copy response to client
	h.logToChan(requestID, latencyMs, upstream.Name, resp.StatusCode, retryAttempt, retryCount)

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
	return nil, resp.StatusCode
}

func (h *ProxyHandler) proxyWithRetry(w http.ResponseWriter, r *http.Request, requestID string) {
	enabled := h.lb.GetEnabled()
	if len(enabled) == 0 {
		h.writeError(w, http.StatusBadGateway, "upstream_error", "No upstream available", 1001)
		return
	}

	var lastUpstream *Upstream
	var lastErr error
	retryCount := 0
	delay := baseDelay

	for attempt := 0; attempt <= maxRetries; attempt++ {
		upstream := h.lb.SelectNext(lastUpstream)
		if upstream == nil {
			upstream = enabled[0]
		}

		retryable, statusCode := h.proxyRequest(w, r, upstream, requestID, attempt, retryCount)
		if retryable == nil {
			return // success
		}
		lastErr = retryable
		lastUpstream = upstream

		// Check if retryable per D-01: only timeout, 5xx, or 429
		if !isRetryable(lastErr, statusCode) {
			break
		}

		if attempt < maxRetries {
			time.Sleep(delay)
			delay *= 2
			if delay > maxDelay {
				delay = maxDelay
			}
			retryCount++
		}
	}

	// All retries exhausted - return 1001 error
	h.writeError(w, http.StatusBadGateway, "upstream_error", "All upstreams failed", 1001)
}

// isRetryable returns true only for retryable errors per D-01:
// - Timeout errors (urlErr.Timeout(), context.DeadlineExceeded)
// - 5xx status codes
// - 429 Too Many Requests
// Returns false for all other errors including 4xx (except 429)
func isRetryable(err error, statusCode int) bool {
	if err == nil {
		return false
	}
	// Check for timeout errors
	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr.Timeout() {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	// Check status codes per D-01: 5xx OR 429 (NOT other 4xx)
	if statusCode >= 500 {
		return true
	}
	if statusCode == 429 {
		return true
	}
	return false
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

func (h *ProxyHandler) logToChan(requestID string, latencyMs int64, upstreamName string, statusCode int, retryAttempt, retryCount int) {
	if h.logChan != nil {
		h.logChan <- RequestLog{
			Timestamp:    time.Now(),
			LatencyMs:    latencyMs,
			UpstreamName: upstreamName,
			StatusCode:   statusCode,
			RequestID:    requestID,
			RetryAttempt: retryAttempt,
			RetryCount:   retryCount,
		}
	}
}

// handleMessages is an alias for ServeHTTP for export compatibility
func (h *ProxyHandler) handleMessages(w http.ResponseWriter, r *http.Request) {
	h.ServeHTTP(w, r)
}
