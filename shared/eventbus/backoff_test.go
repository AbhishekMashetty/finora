package eventbus

import (
	"testing"
	"time"
)

func TestBackoffForAttempt(t *testing.T) {
	tests := []struct {
		name         string
		numDelivered uint64
		want         time.Duration
	}{
		{"first failed attempt uses the first backoff step", 1, 1 * time.Second},
		{"second failed attempt uses the second step", 2, 5 * time.Second},
		{"third failed attempt uses the third step", 3, 15 * time.Second},
		{"fourth failed attempt uses the fourth step", 4, 30 * time.Second},
		{"fifth failed attempt uses the fifth step", 5, 1 * time.Minute},
		{"sixth failed attempt uses the last step", 6, 5 * time.Minute},
		{"beyond the schedule clamps to the last step", 100, 5 * time.Minute},
		{"a zero value (should never happen in practice) still returns the first step, not a panic", 0, 1 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := backoffForAttempt(tt.numDelivered)
			if got != tt.want {
				t.Errorf("backoffForAttempt(%d) = %v, want %v", tt.numDelivered, got, tt.want)
			}
		})
	}
}
