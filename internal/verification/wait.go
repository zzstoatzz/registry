package verification

import (
	"context"
	"time"
)

// WaitWithContext waits for the specified duration with context cancellation support
// Returns true if the timer completed normally, false if context was canceled
func WaitWithContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		// Timer fired normally
		return true
	case <-ctx.Done():
		// Context canceled
		return false
	}
}
