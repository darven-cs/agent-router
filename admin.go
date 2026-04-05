package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// AdminStatus represents the service status response per D-17
type AdminStatus struct {
	ServiceName     string                  `json:"service_name"`
	Version         string                  `json:"version"`
	Uptime          string                  `json:"uptime"`
	TotalRequests   int64                   `json:"total_requests"`
	TotalTokensIn   int64                   `json:"total_tokens_in"`
	TotalTokensOut  int64                   `json:"total_tokens_out"`
	PerUpstream     map[string]UpstreamStats `json:"per_upstream_counts"`
	EnabledChannels []string                `json:"enabled_channels"`
}

// UpstreamStats holds per-upstream aggregated statistics
type UpstreamStats struct {
	RequestCount   int   `json:"request_count"`
	TotalTokensIn  int64 `json:"total_tokens_in"`
	TotalTokensOut int64 `json:"total_tokens_out"`
}

// writeAdminError writes a JSON error response for admin endpoints
func writeAdminError(w http.ResponseWriter, status int, errType, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	errResp := map[string]interface{}{
		"error": map[string]interface{}{
			"type":    errType,
			"message": message,
		},
	}
	json.NewEncoder(w).Encode(errResp)
}

// handleAdminReload triggers a hot config reload via SIGHUP mechanism
// POST /admin/reload - requires authentication
func handleAdminReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAdminError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST allowed")
		return
	}

	// Auth check (same as /v1/messages - D-19)
	token := r.Header.Get("x-api-key")
	if token == "" {
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}
	if token != cfg.Service.APIKey {
		writeAdminError(w, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}

	if err := doReload(); err != nil {
		writeAdminError(w, http.StatusInternalServerError, "reload_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "reloaded"})
}

// handleAdminStatus returns comprehensive service status
// GET /admin/status - requires authentication
func handleAdminStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAdminError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET allowed")
		return
	}

	// Auth check (same as /v1/messages - D-19)
	token := r.Header.Get("x-api-key")
	if token == "" {
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}
	if token != cfg.Service.APIKey {
		writeAdminError(w, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}

	// Get in-memory stats (thread-safe via GetCounts)
	totalReqs, totalIn, totalOut := Stats.GetCounts()

	// Query per-upstream counts from SQLite
	perUpstream := make(map[string]UpstreamStats)
	if db != nil {
		var upstreamCounts []struct {
			UpstreamName string
			Count        int
			TokensIn     int64
			TokensOut    int64
		}
		db.Model(&UsageLog{}).
			Select("upstream_name, COUNT(*) as count, SUM(input_tokens) as tokens_in, SUM(output_tokens) as tokens_out").
			Group("upstream_name").
			Scan(&upstreamCounts)

		for _, uc := range upstreamCounts {
			perUpstream[uc.UpstreamName] = UpstreamStats{
				RequestCount:   uc.Count,
				TotalTokensIn:  uc.TokensIn,
				TotalTokensOut: uc.TokensOut,
			}
		}
	}

	// Get enabled channels
	enabledChannels := make([]string, 0)
	for _, us := range sharedUpstreams.GetAll() {
		if us.Enabled {
			enabledChannels = append(enabledChannels, us.Name)
		}
	}

	status := AdminStatus{
		ServiceName:     cfg.Service.Name,
		Version:         cfg.Service.Version,
		Uptime:          time.Since(startTime).String(),
		TotalRequests:   totalReqs,
		TotalTokensIn:   totalIn,
		TotalTokensOut:  totalOut,
		PerUpstream:     perUpstream,
		EnabledChannels: enabledChannels,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}
