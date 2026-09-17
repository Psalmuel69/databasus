package storage_files

import "time"

// The single policy the store and the worker read at construction: every deadline
// plus how many rows one claim may take. Tests shrink it instead of waiting, so
// nothing here is read from a package-level variable at call time.
type Timings struct {
	// CommitWindow is how long a file may stay recorded but unclaimed by any
	// catalog transaction before cleanup assumes its publisher is never coming.
	CommitWindow time.Duration
	// AttemptLease bounds one provider deletion and is also how far a claim
	// pushes not_before, so a stalled call cannot hold its row forever.
	AttemptLease   time.Duration
	RetryBaseDelay time.Duration
	RetryMaxDelay  time.Duration
	WorkerTick     time.Duration
	ClaimBatchSize int
}

func ProductionTimings() Timings {
	return Timings{
		CommitWindow:   15 * time.Minute,
		AttemptLease:   2 * time.Minute,
		RetryBaseDelay: 30 * time.Second,
		RetryMaxDelay:  time.Hour,
		WorkerTick:     time.Minute,
		ClaimBatchSize: 50,
	}
}
