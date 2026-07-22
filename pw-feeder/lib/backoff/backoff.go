package backoff

import "time"

type (
	// BackerOff tracks retry attempts and calculates the delay before each retry.
	BackerOff struct {
		// method calculates a delay from the current attempt number.
		method Method
		// resetAfter specifies how long inactivity must last before attempts reset.
		resetAfter time.Duration
		// lastAttempt records when the most recent delay was calculated.
		lastAttempt time.Time
		// attempt is the number passed to method for the next delay calculation.
		attempt int64
	}

	// Method calculates a backoff duration for a retry attempt.
	Method func(attempt int64) time.Duration

	// Option configures a BackerOff.
	Option func(*BackerOff)
)

// WithMethod returns an Option that sets the backoff calculation method.
func WithMethod(method Method) Option {
	return func(bo *BackerOff) {
		bo.method = method
	}
}

// WithResetAfter returns an Option that sets the inactivity period after which
// the retry attempt counter resets.
func WithResetAfter(d time.Duration) Option {
	return func(bo *BackerOff) {
		bo.resetAfter = d
	}
}

// DefaultMethodExponentialBackoff returns the default delay for an attempt.
// The first attempt has no delay, and subsequent delays are capped at 30 seconds.
func DefaultMethodExponentialBackoff(attempt int64) time.Duration {
	if attempt == 0 {
		return 0
	}
	return min(time.Duration(attempt*attempt)*time.Second, time.Second*30)
}

// New returns a BackerOff configured with the supplied options.
func New(opts ...Option) *BackerOff {
	// Set the defaults.
	bo := &BackerOff{
		resetAfter: 30 * time.Second,
		method:     DefaultMethodExponentialBackoff,
	}
	for _, opt := range opts {
		opt(bo)
	}
	return bo
}

// BackOff returns the delay for the next retry and records the attempt.
// It resets the attempt counter after the configured period of inactivity.
func (bo *BackerOff) BackOff() time.Duration {
	if bo.lastAttempt.Add(bo.resetAfter).Before(time.Now()) {
		bo.attempt = 0
	}

	sleepyTime := bo.method(bo.attempt)
	bo.attempt++
	bo.lastAttempt = time.Now()

	return sleepyTime
}
