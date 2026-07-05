// Package resilience provides circuit breaker and rate limiter stubs
// for the EASP open source core. The commercial version includes real
// circuit breaker state machines and token bucket rate limiters.
package resilience

import "time"

// CircuitBreakerConfig holds circuit breaker configuration.
type CircuitBreakerConfig struct {
	FailureThreshold int
	SuccessThreshold int
	Timeout          time.Duration
	HalfOpenMaxCalls int
}

// DefaultCircuitBreakerConfig returns a default configuration.
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:          30 * time.Second,
		HalfOpenMaxCalls: 3,
	}
}

// CircuitBreaker protects a service from cascading failures.
// In the open source version, this is a pass-through stub.
type CircuitBreaker struct {
	config CircuitBreakerConfig
}

// Execute runs fn, always succeeding in the open source version.
func (cb *CircuitBreaker) Execute(fn func() error) error {
	return fn()
}

// CircuitBreakerManager manages multiple circuit breakers.
type CircuitBreakerManager struct {
	config CircuitBreakerConfig
}

// NewCircuitBreakerManager creates a new manager.
func NewCircuitBreakerManager(config CircuitBreakerConfig) *CircuitBreakerManager {
	return &CircuitBreakerManager{config: config}
}

// GetOrCreate returns a new CircuitBreaker for the given key.
func (m *CircuitBreakerManager) GetOrCreate(_ string) *CircuitBreaker {
	return &CircuitBreaker{config: m.config}
}

// GetAll returns all circuit breakers (empty in open source).
func (m *CircuitBreakerManager) GetAll() map[string]interface{} {
	return map[string]interface{}{}
}

// RateLimiter is the interface for rate limiting.
type RateLimiter interface {
	Allow() bool
}

// TokenBucketLimiter implements RateLimiter using token bucket algorithm.
// In the open source version, this always allows requests.
type TokenBucketLimiter struct {
	name string
}

// NewTokenBucketLimiter creates a new token bucket limiter.
func NewTokenBucketLimiter(name string, _ float64, _ int) *TokenBucketLimiter {
	return &TokenBucketLimiter{name: name}
}

// Allow always returns true in the open source version.
func (l *TokenBucketLimiter) Allow() bool { return true }

// RateLimiterManager manages multiple rate limiters.
type RateLimiterManager struct {
	limiters map[string]RateLimiter
}

// NewRateLimiterManager creates a new manager.
func NewRateLimiterManager() *RateLimiterManager {
	return &RateLimiterManager{limiters: make(map[string]RateLimiter)}
}

// GetOrCreate returns a RateLimiter for the given key.
func (m *RateLimiterManager) GetOrCreate(key string) RateLimiter {
	if l, ok := m.limiters[key]; ok {
		return l
	}
	l := NewTokenBucketLimiter(key, 100, 100)
	m.limiters[key] = l
	return l
}

// Get returns a RateLimiter for the given key, and whether it was found.
func (m *RateLimiterManager) Get(key string) (RateLimiter, bool) {
	l, ok := m.limiters[key]
	return l, ok
}

// GetAll returns all rate limiters.
func (m *RateLimiterManager) GetAll() map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m.limiters {
		result[k] = v
	}
	return result
}

// Register adds a rate limiter.
func (m *RateLimiterManager) Register(key string, l RateLimiter) {
	m.limiters[key] = l
}
