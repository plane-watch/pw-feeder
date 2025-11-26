package backoff

import "time"

type (
	BackerOff struct {
		method      Method
		resetAfter  time.Duration
		lastAttempt time.Time
		attempt     int64
	}

	Method func(attempt int64) time.Duration
	Option func(*BackerOff)
)

func WithMethod(method Method) Option {
	return func(bo *BackerOff) {
		bo.method = method
	}
}

func WithResetAfter(d time.Duration) Option {
	return func(bo *BackerOff) {
		bo.resetAfter = d
	}
}

func DefaultMethodExponentialBackoff(attempt int64) time.Duration {
	if attempt == 0 {
		return 0
	}
	return min(time.Duration(attempt*attempt)*time.Second, time.Second*30)
}

func New(opts ...Option) *BackerOff {
	// set defaults
	bo := &BackerOff{
		resetAfter: 30 * time.Second,
		method:     DefaultMethodExponentialBackoff,
	}
	for _, opt := range opts {
		opt(bo)
	}
	return bo
}

func (bo *BackerOff) BackOff() time.Duration {
	if bo.lastAttempt.Add(bo.resetAfter).Before(time.Now()) {
		bo.attempt = 0
	}

	sleepyTime := bo.method(bo.attempt)
	bo.attempt++
	bo.lastAttempt = time.Now()

	return sleepyTime
}
