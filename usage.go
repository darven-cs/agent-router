package main

import (
	"fmt"
	"os"
	"sync"
	"time"

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

var stats = &UsageStats{}

// initDB initializes SQLite database with WAL mode
func initDB(dbPath string) (*gorm.DB, error) {
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

// StartUsageWorker drains usageChan and writes to SQLite asynchronously
func StartUsageWorker(db *gorm.DB, usageChan <-chan RequestLog) {
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
				// Update in-memory stats (fire-and-forget, non-blocking)
				stats.mu.Lock()
				stats.totalRequests++
				stats.totalTokensIn += int64(log.InputTokens)
				stats.totalTokensOut += int64(log.OutputTokens)
				stats.mu.Unlock()
			}
		}
	}()
}
