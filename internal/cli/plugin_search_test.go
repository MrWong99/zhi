package cli

import (
	"testing"
)

func TestFormatDownloads(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0"},
		{42, "42"},
		{999, "999"},
		{1000, "1.0k"},
		{1500, "1.5k"},
		{12450, "12.4k"},
		{999999, "1000.0k"},
		{1000000, "1.0M"},
		{28700000, "28.7M"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatDownloads(tt.input)
			if got != tt.want {
				t.Errorf("formatDownloads(%d) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
