package app

import "time"

// A heartbeat too far in the future is not proof of a live worker. Keep API
// health and the source-profile catalog consistent when clocks disagree.
func workerHeartbeatOnline(beat time.Time) bool {
	age := time.Since(beat)
	return age >= -5*time.Second && age < 30*time.Second
}
