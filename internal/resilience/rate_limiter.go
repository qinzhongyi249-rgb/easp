package resilience

import (
	"fmt"
	"sync"
	"time"
)

// RateLimiter 限流器接口
type RateLimiter interface {
	Allow() bool
	AllowN(n int) bool
	Wait(timeout time.Duration) error
	GetStats() map[string]interface{}
}

// TokenBucketLimiter 令牌桶限流器
type TokenBucketLimiter struct {
	name       string
	rate       float64 // 每秒生成的令牌数
	capacity   int     // 桶容量
	tokens     float64 // 当前令牌数
	lastRefill time.Time
	mu         sync.Mutex
}

// NewTokenBucketLimiter 创建令牌桶限流器
func NewTokenBucketLimiter(name string, rate float64, capacity int) *TokenBucketLimiter {
	return &TokenBucketLimiter{
		name:       name,
		rate:       rate,
		capacity:   capacity,
		tokens:     float64(capacity),
		lastRefill: time.Now(),
	}
}

// Allow 检查是否允许请求
func (l *TokenBucketLimiter) Allow() bool {
	return l.AllowN(1)
}

// AllowN 检查是否允许N个请求
func (l *TokenBucketLimiter) AllowN(n int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.refill()

	if l.tokens >= float64(n) {
		l.tokens -= float64(n)
		return true
	}
	return false
}

// Wait 等待直到允许或超时
func (l *TokenBucketLimiter) Wait(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if l.Allow() {
			return nil
		}
		if time.Now().After(deadline) {
			return ErrRateLimitExceeded
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// refill 补充令牌
func (l *TokenBucketLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(l.lastRefill).Seconds()
	l.tokens += elapsed * l.rate
	if l.tokens > float64(l.capacity) {
		l.tokens = float64(l.capacity)
	}
	l.lastRefill = now
}

// GetStats 获取统计信息
func (l *TokenBucketLimiter) GetStats() map[string]interface{} {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.refill()
	return map[string]interface{}{
		"name":     l.name,
		"rate":     l.rate,
		"capacity": l.capacity,
		"tokens":   l.tokens,
	}
}

// SlidingWindowLimiter 滑动窗口限流器
type SlidingWindowLimiter struct {
	name     string
	limit    int
	window   time.Duration
	requests []time.Time
	mu       sync.Mutex
}

// NewSlidingWindowLimiter 创建滑动窗口限流器
func NewSlidingWindowLimiter(name string, limit int, window time.Duration) *SlidingWindowLimiter {
	return &SlidingWindowLimiter{
		name:     name,
		limit:    limit,
		window:   window,
		requests: make([]time.Time, 0),
	}
}

// Allow 检查是否允许请求
func (l *SlidingWindowLimiter) Allow() bool {
	return l.AllowN(1)
}

// AllowN 检查是否允许N个请求
func (l *SlidingWindowLimiter) AllowN(n int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-l.window)

	// 清理过期请求
	validRequests := make([]time.Time, 0)
	for _, t := range l.requests {
		if t.After(windowStart) {
			validRequests = append(validRequests, t)
		}
	}
	l.requests = validRequests

	if len(l.requests)+n <= l.limit {
		for i := 0; i < n; i++ {
			l.requests = append(l.requests, now)
		}
		return true
	}
	return false
}

// Wait 等待直到允许或超时
func (l *SlidingWindowLimiter) Wait(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if l.Allow() {
			return nil
		}
		if time.Now().After(deadline) {
			return ErrRateLimitExceeded
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// GetStats 获取统计信息
func (l *SlidingWindowLimiter) GetStats() map[string]interface{} {
	l.mu.Lock()
	defer l.mu.Unlock()
	return map[string]interface{}{
		"name":    l.name,
		"limit":   l.limit,
		"window":  l.window.String(),
		"current": len(l.requests),
	}
}

// ErrRateLimitExceeded 限流错误
var ErrRateLimitExceeded = &RateLimitError{Message: "rate limit exceeded"}

type RateLimitError struct {
	Message string
}

func (e *RateLimitError) Error() string {
	return e.Message
}

// MultiLevelLimiter 多级限流器
type MultiLevelLimiter struct {
	limiters []RateLimiter
}

// NewMultiLevelLimiter 创建多级限流器
func NewMultiLevelLimiter(limiters ...RateLimiter) *MultiLevelLimiter {
	return &MultiLevelLimiter{limiters: limiters}
}

// Allow 检查是否允许请求
func (l *MultiLevelLimiter) Allow() bool {
	for _, limiter := range l.limiters {
		if !limiter.Allow() {
			return false
		}
	}
	return true
}

// AllowN 检查是否允许N个请求
func (l *MultiLevelLimiter) AllowN(n int) bool {
	for _, limiter := range l.limiters {
		if !limiter.AllowN(n) {
			return false
		}
	}
	return true
}

// Wait 等待直到允许或超时
func (l *MultiLevelLimiter) Wait(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if l.Allow() {
			return nil
		}
		if time.Now().After(deadline) {
			return ErrRateLimitExceeded
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// GetStats 获取统计信息
func (l *MultiLevelLimiter) GetStats() map[string]interface{} {
	stats := make(map[string]interface{})
	for i, limiter := range l.limiters {
		stats[fmt.Sprintf("limiter_%d", i)] = limiter.GetStats()
	}
	return stats
}

// RateLimiterManager 限流器管理器
type RateLimiterManager struct {
	limiters map[string]RateLimiter
	mu       sync.RWMutex
}

// NewRateLimiterManager 创建限流器管理器
func NewRateLimiterManager() *RateLimiterManager {
	return &RateLimiterManager{
		limiters: make(map[string]RateLimiter),
	}
}

// Register 注册限流器
func (m *RateLimiterManager) Register(name string, limiter RateLimiter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.limiters[name] = limiter
}

// Get 获取限流器
func (m *RateLimiterManager) Get(name string) (RateLimiter, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	limiter, ok := m.limiters[name]
	return limiter, ok
}

// GetAll 获取所有限流器统计
func (m *RateLimiterManager) GetAll() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]interface{})
	for name, limiter := range m.limiters {
		result[name] = limiter.GetStats()
	}
	return result
}
