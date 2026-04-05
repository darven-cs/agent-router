package admin

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"agent-router/internal/config"
	"agent-router/internal/storage"
	"agent-router/internal/upstream"

	"gorm.io/gorm"
)

// AdminStatus represents the service status response per D-17
type AdminStatus struct {
	ServiceName     string                   `json:"service_name"`
	Version         string                   `json:"version"`
	Uptime          string                   `json:"uptime"`
	TotalRequests   int64                    `json:"total_requests"`
	TotalTokensIn   int64                    `json:"total_tokens_in"`
	TotalTokensOut  int64                    `json:"total_tokens_out"`
	PerUpstream     map[string]UpstreamStats `json:"per_upstream_counts"`
	EnabledChannels []string                 `json:"enabled_channels"`
}

// UpstreamStats holds per-upstream aggregated statistics
type UpstreamStats struct {
	RequestCount   int   `json:"request_count"`
	TotalTokensIn  int64 `json:"total_tokens_in"`
	TotalTokensOut int64 `json:"total_tokens_out"`
}

// AdminHandler handles admin API requests
type AdminHandler struct {
	cfg             *config.Config
	db              *gorm.DB
	stats           *storage.UsageStats
	sharedUpstreams *upstream.SharedUpstreams
	startTime       time.Time
	reloadFn        func() error
}

// NewAdminHandler creates a new admin handler receiving all dependencies through constructor
func NewAdminHandler(cfg *config.Config, db *gorm.DB, stats *storage.UsageStats, su *upstream.SharedUpstreams, startTime time.Time, reloadFn func() error) *AdminHandler {
	return &AdminHandler{
		cfg:             cfg,
		db:              db,
		stats:           stats,
		sharedUpstreams: su,
		startTime:       startTime,
		reloadFn:        reloadFn,
	}
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

// ServeHTTP routes admin requests to appropriate handlers
func (h *AdminHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/admin/status":
		h.HandleStatus(w, r)
	case r.URL.Path == "/admin/reload":
		h.HandleReload(w, r)
	default:
		http.NotFound(w, r)
	}
}

// HandleReload triggers a hot config reload via SIGHUP mechanism
// POST /admin/reload - requires authentication
func (h *AdminHandler) HandleReload(w http.ResponseWriter, r *http.Request) {
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
	if token != h.cfg.Service.APIKey {
		writeAdminError(w, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}

	if err := h.reloadFn(); err != nil {
		writeAdminError(w, http.StatusInternalServerError, "reload_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "reloaded"})
}

// HandleStatus returns comprehensive service status
// GET /admin/status - requires authentication
func (h *AdminHandler) HandleStatus(w http.ResponseWriter, r *http.Request) {
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
	if token != h.cfg.Service.APIKey {
		writeAdminError(w, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}

	// Get in-memory stats (thread-safe via GetCounts)
	totalReqs, totalIn, totalOut := h.stats.GetCounts()

	// Query per-upstream counts from SQLite
	perUpstream := make(map[string]UpstreamStats)
	if h.db != nil {
		var upstreamCounts []struct {
			UpstreamName string
			Count        int
			TokensIn     int64
			TokensOut    int64
		}
		h.db.Model(&storage.UsageLog{}).
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
	for _, us := range h.sharedUpstreams.GetAll() {
		if us.Enabled {
			enabledChannels = append(enabledChannels, us.Name)
		}
	}

	status := AdminStatus{
		ServiceName:     h.cfg.Service.Name,
		Version:         h.cfg.Service.Version,
		Uptime:          time.Since(h.startTime).String(),
		TotalRequests:   totalReqs,
		TotalTokensIn:   totalIn,
		TotalTokensOut:  totalOut,
		PerUpstream:     perUpstream,
		EnabledChannels: enabledChannels,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}
