package server

import (
	"testing"
	"time"
)

func TestParseSyncInterval(t *testing.T) {
	tests := []struct {
		schedule string
		want     time.Duration
	}{
		{"0 2 * * *", 24 * time.Hour},      // daily
		{"0 */6 * * *", 6 * time.Hour},     // every 6 hours
		{"*/30 * * * *", 30 * time.Minute}, // every 30 minutes
		{"*/15 * * * *", 15 * time.Minute}, // every 15 minutes
		{"0 */12 * * *", 12 * time.Hour},   // every 12 hours
		{"invalid", 24 * time.Hour},        // fallback
		{"", 24 * time.Hour},               // empty
		{"0 * * * *", 24 * time.Hour},      // every hour (no interval pattern)
	}

	for _, tt := range tests {
		t.Run(tt.schedule, func(t *testing.T) {
			got := parseSyncInterval(tt.schedule)
			if got != tt.want {
				t.Errorf("parseSyncInterval(%q) = %v, want %v", tt.schedule, got, tt.want)
			}
		})
	}
}
