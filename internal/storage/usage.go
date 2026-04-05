package storage

import (
	"fmt"
	"os"
	"sync"
	"time"

	"agent-router/internal/proxy"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// UsageLog GORM model stores per-request usage data
type UsageLog struct {
	ID           uint      `gorm:"primaryKey"`
	Timestamp    time.Time `gorm:"index:idx_timestamp_upstream"`
	RequestID    string    `gorm:"index"`
	UpstreamName string    `gorm:"index:idx_timestamp_upstream"`
	InputTokens  int       `gorm:"default:0"`
	OutputTokens int       `gorm:"default:0"`
	LatencyMs    int64
	StatusCode   int
}

// UsageStats thread-safe in-memory counters
type UsageStats struct {
	mu             sync.RWMutex
	totalRequests  int64
	totalTokensIn  int64
	totalTokensOut int64
}

// Record increments stats for a single request
func (s *UsageStats) Record(inputTokens, outputTokens int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.totalRequests++
	s.totalTokensIn += int64(inputTokens)
	s.totalTokensOut += int64(outputTokens)
}

// GetCounts returns current total stats (thread-safe)
func (s *UsageStats) GetCounts() (totalRequests, totalTokensIn, totalTokensOut int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.totalRequests, s.totalTokensIn, s.totalTokensOut
}

// Stats is the global usage statistics instance
var Stats = &UsageStats{}

// InitDB initializes SQLite database with WAL mode
func InitDB(dbPath string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// Enable WAL mode for better concurrent performance
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA synchronous=NORMAL")

	// Auto-migrate schema
	db.AutoMigrate(&UsageLog{})

	return db, nil
}

// StartUsageWorker starts a goroutine that drains usageChan and writes to SQLite
func StartUsageWorker(db *gorm.DB, usageChan <-chan proxy.RequestLog) {
	go func() {
		for log := range usageChan {
			usageLog := UsageLog{
				Timestamp:    log.Timestamp,
				RequestID:    log.RequestID,
				UpstreamName: log.UpstreamName,
				InputTokens:  log.InputTokens,
				OutputTokens: log.OutputTokens,
				LatencyMs:    log.LatencyMs,
				StatusCode:   log.StatusCode,
			}

			if err := db.Create(&usageLog).Error; err != nil {
				fmt.Fprintf(os.Stderr, "Usage write error: %v\n", err)
			} else {
				Stats.Record(log.InputTokens, log.OutputTokens)
			}
		}
	}()
}
