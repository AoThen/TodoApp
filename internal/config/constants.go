package config

import "time"

// Token-related constants
const (
	AccessTokenDuration  = 15 * time.Minute
	RefreshTokenDuration = 7 * 24 * time.Hour // 7 days
)

// Rate limiting constants
const (
	MaxLoginAttempts   = 5
	LoginAttemptWindow = 15 * time.Minute
	LockoutDuration    = 30 * time.Minute
)

// Sync-related constants
const (
	SyncInterval = 30 * time.Second
)

// Task-related constants
const (
	UndoWindowDuration = 30 * time.Second // Time window to undo task deletion
)

// HTTP server constants
const (
	ReadTimeout  = 30 * time.Second
	WriteTimeout = 60 * time.Second
	IdleTimeout  = 120 * time.Second
)

// Database-related constants
const (
	CleanupInterval = 1 * time.Hour
)
