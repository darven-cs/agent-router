package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"agent-router/internal/upstream"
)

const (
	baseDelay  = 1 * time.Second
	maxRetries = 3
	maxDelay   = 4 * time.Second
)

// RequestLog records a single request's details
type RequestLog struct {
	Timestamp    time.Time
	LatencyMs    int64
	UpstreamName string
	StatusCode   int
	RequestID    string
	RetryAttempt int // Current retry attempt (0=initial, 1+=retries)
	RetryCount   int // Total retries for this request
	InputTokens  int // Tokens in request
	OutputTokens int // Tokens in response
}

// ProxyHandler manages HTTP proxying with authentication
type ProxyHandler struct {
	lb           *upstream.LoadBalancer
	apiKey       string
	defaultModel string
	logChan      chan<- RequestLog // For TUI display (no tokens)
	usageChan    chan<- RequestLog // For SQLite persistence (with tokens)
}

// NewProxyHandler creates a new proxy handler
func NewProxyHandler(lb *upstream.LoadBalancer, apiKey string, defaultModel string, logChan chan RequestLog, usageChan chan RequestLog) *ProxyHandler {
	return &ProxyHandler{
		lb:           lb,
		apiKey:       apiKey,
		defaultModel: defaultModel,
		logChan:      logChan,
		usageChan:    usageChan,
	}
}

// SetLoadBalancer replaces the load balancer instance (used by doReload)
func (h *ProxyHandler) SetLoadBalancer(lb *upstream.LoadBalancer) {
	h.lb = lb
}

// SetDefaultModel updates the default model (used by doReload and TUI callbacks)
func (h *ProxyHandler) SetDefaultModel(model string) {
	h.defaultModel = model
}

// transformModelName replaces the model name in request JSON.
// It uses the defaultModel if set, otherwise falls back to upstream-specific model.
func transformModelName(body []byte, defaultModel, upstreamModel string) []byte {
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return body
	}

	if _, ok := data["model"].(string); ok {
		if defaultModel != "" {
			// Use service default model
			data["model"] = defaultModel
		} else if upstreamModel != "" {
			// Fall back to upstream-specific model
			data["model"] = upstreamModel
		}
	}

	out, _ := json.Marshal(data)
	return out
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

func (h *ProxyHandler) proxyRequest(w http.ResponseWriter, r *http.Request, upstream *upstream.Upstream, requestID string, retryAttempt, retryCount int) (error, int) {
	start := time.Now()

	// Read and transform request body if upstream has a custom model
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err, 0
	}

	// Transform model name if upstream specifies one
	if upstream.Model != "" || h.defaultModel != "" {
		bodyBytes = transformModelName(bodyBytes, h.defaultModel, upstream.Model)
	}

	// Build upstream URL - append /v1/messages if not present
	upstreamURL := upstream.URL
	if !strings.HasSuffix(upstreamURL, "/v1/messages") {
		if !strings.HasSuffix(upstreamURL, "/") {
			upstreamURL += "/"
		}
		upstreamURL += "v1/messages"
	}

	// Create upstream request with transformed body
	req, err := http.NewRequest(http.MethodPost, upstreamURL, strings.NewReader(string(bodyBytes)))
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

	// Buffer the response body for token extraction
	bodyBytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		h.logToChan(requestID, latencyMs, upstream.Name, resp.StatusCode, retryAttempt, retryCount)
		return err, resp.StatusCode
	}

	// Extract usage tokens from Claude API response
	var inputTokens, outputTokens int
	var respData map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &respData); err == nil {
		if usage, ok := respData["usage"].(map[string]interface{}); ok {
			if v, ok := usage["input_tokens"].(float64); ok {
				inputTokens = int(v)
			}
			if v, ok := usage["output_tokens"].(float64); ok {
				outputTokens = int(v)
			}
		}
	}

	// Log with tokens (both TUI and SQLite channels)
	h.logToChan(requestID, latencyMs, upstream.Name, resp.StatusCode, retryAttempt, retryCount)
	h.logToChanWithTokens(requestID, latencyMs, upstream.Name, resp.StatusCode, retryAttempt, retryCount, inputTokens, outputTokens)

	// Write response to client
	w.WriteHeader(resp.StatusCode)
	w.Write(bodyBytes)
	return nil, resp.StatusCode
}

func (h *ProxyHandler) proxyWithRetry(w http.ResponseWriter, r *http.Request, requestID string) {
	enabled := h.lb.GetEnabled()
	if len(enabled) == 0 {
		h.writeError(w, http.StatusBadGateway, "upstream_error", "No upstream available", 1001)
		return
	}

	var lastUpstream *upstream.Upstream
	var lastErr error
	retryCount := 0
	delay := baseDelay

	// Primary upstream check: if set, try it first
	primary := h.lb.GetPrimary()
	if primary != nil {
		// Verify primary is still in enabled list
		for _, u := range enabled {
			if u == primary && u.Enabled {
				lastUpstream = primary
				break
			}
		}
	}
	// If no primary or primary not valid, use normal selection
	if lastUpstream == nil {
		lastUpstream = h.lb.SelectNext(nil)
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		upstream := lastUpstream
		if attempt > 0 {
			upstream = h.lb.SelectNext(lastUpstream)
		}
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

func (h *ProxyHandler) logToChanWithTokens(requestID string, latencyMs int64, upstreamName string, statusCode int, retryAttempt, retryCount, inputTokens, outputTokens int) {
	if h.usageChan != nil {
		h.usageChan <- RequestLog{
			Timestamp:    time.Now(),
			LatencyMs:    latencyMs,
			UpstreamName: upstreamName,
			StatusCode:   statusCode,
			RequestID:    requestID,
			RetryAttempt: retryAttempt,
			RetryCount:   retryCount,
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
		}
	}
}

// handleMessages is an alias for ServeHTTP for export compatibility
func (h *ProxyHandler) handleMessages(w http.ResponseWriter, r *http.Request) {
	h.ServeHTTP(w, r)
}
