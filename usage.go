package main

import (
	"agent-router/internal/proxy"
	"agent-router/internal/storage"

	"gorm.io/gorm"
)

type UsageLog = storage.UsageLog
type UsageStats = storage.UsageStats
type RequestLog = proxy.RequestLog

var Stats = storage.Stats
var initDB = storage.InitDB
var StartUsageWorker = storage.StartUsageWorker

// Type compatibility check: usageChan uses proxy.RequestLog
var _ chan<- proxy.RequestLog = (chan<- RequestLog)(nil)

// Ensure gorm.DB type is available for root package references
var _ = func() *gorm.DB { return nil }
