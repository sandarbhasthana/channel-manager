package infra

// NoopCircuitBreaker is a stub implementation of ports.CircuitBreaker
// that always allows requests. Replace with gobreaker or similar when ready.
type NoopCircuitBreaker struct{}

// NewNoopCircuitBreaker creates a new no-op circuit breaker.
func NewNoopCircuitBreaker() *NoopCircuitBreaker {
	return &NoopCircuitBreaker{}
}

// Allow always permits the call and returns a no-op done callback.
func (b *NoopCircuitBreaker) Allow(_ string) (func(bool), error) {
	return func(_ bool) {}, nil
}
