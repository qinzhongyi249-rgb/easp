package resilience

import (
	"fmt"
	"sync"
	"time"
)

// CircuitBreakerState 熔断器状态
type CircuitBreakerState string

const (
	StateClosed   CircuitBreakerState = "closed"    // 正常状态
	StateOpen     CircuitBreakerState = "open"      // 熔断状态
	StateHalfOpen CircuitBreakerState = "half_open" // 半开状态
)

// CircuitBreakerConfig 熔断器配置
type CircuitBreakerConfig struct {
	FailureThreshold int           `json:"failure_threshold"` // 失败阈值
	SuccessThreshold int           `json:"success_threshold"` // 成功阈值(半开->关闭)
	Timeout          time.Duration `json:"timeout"`           // 熔断超时时间
	MaxRequests      int           `json:"max_requests"`      // 半开状态最大请求数
}

// DefaultCircuitBreakerConfig 默认配置
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 3,
		Timeout:          60 * time.Second,
		MaxRequests:      1,
	}
}

// CircuitBreaker 熔断器
type CircuitBreaker struct {
	name          string
	config        CircuitBreakerConfig
	state         CircuitBreakerState
	failureCount  int
	successCount  int
	lastFailure   time.Time
	lastStateTime time.Time
	mu            sync.RWMutex
	onStateChange func(from, to CircuitBreakerState)
}

// NewCircuitBreaker 创建熔断器
func NewCircuitBreaker(name string, config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		name:          name,
		config:        config,
		state:         StateClosed,
		lastStateTime: time.Now(),
	}
}

// Execute 执行操作
func (cb *CircuitBreaker) Execute(fn func() error) error {
	if !cb.allowRequest() {
		return fmt.Errorf("circuit breaker %s is open, request rejected", cb.name)
	}

	err := fn()
	cb.recordResult(err)
	return err
}

// allowRequest 是否允许请求
func (cb *CircuitBreaker) allowRequest() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// 检查是否超时，可以进入半开状态
		if time.Since(cb.lastFailure) > cb.config.Timeout {
			cb.mu.RUnlock()
			cb.mu.Lock()
			cb.transitionTo(StateHalfOpen)
			cb.mu.Unlock()
			cb.mu.RLock()
			return true
		}
		return false
	case StateHalfOpen:
		return cb.successCount+cb.failureCount < cb.config.MaxRequests
	default:
		return false
	}
}

// recordResult 记录结果
func (cb *CircuitBreaker) recordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failureCount++
		cb.lastFailure = time.Now()
		if cb.state == StateClosed && cb.failureCount >= cb.config.FailureThreshold {
			cb.transitionTo(StateOpen)
		} else if cb.state == StateHalfOpen {
			cb.transitionTo(StateOpen)
		}
	} else {
		cb.successCount++
		if cb.state == StateHalfOpen && cb.successCount >= cb.config.SuccessThreshold {
			cb.transitionTo(StateClosed)
		}
	}
}

// transitionTo 状态转换
func (cb *CircuitBreaker) transitionTo(state CircuitBreakerState) {
	if cb.state == state {
		return
	}
	from := cb.state
	cb.state = state
	cb.lastStateTime = time.Now()

	if state == StateClosed {
		cb.failureCount = 0
		cb.successCount = 0
	} else if state == StateHalfOpen {
		cb.successCount = 0
	}

	if cb.onStateChange != nil {
		go cb.onStateChange(from, state)
	}
}

// GetState 获取当前状态
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats 获取统计信息
func (cb *CircuitBreaker) GetStats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return map[string]interface{}{
		"name":          cb.name,
		"state":         string(cb.state),
		"failure_count": cb.failureCount,
		"success_count": cb.successCount,
		"last_failure":  cb.lastFailure,
		"last_state_change": cb.lastStateTime,
	}
}

// OnStateChange 设置状态变更回调
func (cb *CircuitBreaker) OnStateChange(fn func(from, to CircuitBreakerState)) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.onStateChange = fn
}

// Reset 重置熔断器
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = StateClosed
	cb.failureCount = 0
	cb.successCount = 0
	cb.lastStateTime = time.Now()
}

// CircuitBreakerManager 熔断器管理器
type CircuitBreakerManager struct {
	breakers map[string]*CircuitBreaker
	config   CircuitBreakerConfig
	mu       sync.RWMutex
}

// NewCircuitBreakerManager 创建熔断器管理器
func NewCircuitBreakerManager(config CircuitBreakerConfig) *CircuitBreakerManager {
	return &CircuitBreakerManager{
		breakers: make(map[string]*CircuitBreaker),
		config:   config,
	}
}

// GetOrCreate 获取或创建熔断器
func (m *CircuitBreakerManager) GetOrCreate(name string) *CircuitBreaker {
	m.mu.RLock()
	if cb, ok := m.breakers[name]; ok {
		m.mu.RUnlock()
		return cb
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()
	// 双重检查
	if cb, ok := m.breakers[name]; ok {
		return cb
	}
	cb := NewCircuitBreaker(name, m.config)
	m.breakers[name] = cb
	return cb
}

// GetAll 获取所有熔断器状态
func (m *CircuitBreakerManager) GetAll() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]interface{})
	for name, cb := range m.breakers {
		result[name] = cb.GetStats()
	}
	return result
}
