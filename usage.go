package main

import (
	"fmt"
	"os"

	"agent-router/internal/storage"

	"gorm.io/gorm"
)

type UsageLog = storage.UsageLog
type UsageStats = storage.UsageStats

var Stats = storage.Stats
var initDB = storage.InitDB

// StartUsageWorker remains here until proxy.RequestLog is available (Plan 02)
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
				// Update in-memory stats using exported method
				Stats.Record(log.InputTokens, log.OutputTokens)
			}
		}
	}()
}
