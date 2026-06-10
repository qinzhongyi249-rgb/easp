package middleware

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/easp-platform/easp/internal/database"
	"github.com/easp-platform/easp/internal/resilience"
	"github.com/gin-gonic/gin"
)

// tenantRateLimiter 租户限流器（内存中维护）
type tenantRateLimiter struct {
	limiter   *resilience.SlidingWindowLimiter
	rateLimit int       // 配置值，用于检测变更
	updatedAt time.Time // 上次同步时间
}

var (
	tenantLimiters = make(map[string]*tenantRateLimiter)
	limiterMu      sync.RWMutex
)

// RateLimitMiddleware 租户级限流中间件
// 基于滑动窗口，按 tenant_id 限流
func RateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, exists := c.Get(ContextTenantID)
		if !exists {
			c.Next()
			return
		}

		tid := tenantID.(string)
		limiter := getOrCreateLimiter(tid)
		if limiter == nil {
			// 无限制或获取失败，放行
			c.Next()
			return
		}

		if !limiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "Rate limit exceeded",
				"message": "请求过于频繁，请稍后再试",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// getOrCreateLimiter 获取或创建租户限流器
func getOrCreateLimiter(tenantID string) *resilience.SlidingWindowLimiter {
	limiterMu.RLock()
	tl, exists := tenantLimiters[tenantID]
	limiterMu.RUnlock()

	// 每 30 秒同步一次配置
	if exists && time.Since(tl.updatedAt) < 30*time.Second {
		return tl.limiter
	}

	// 从数据库读取配置
	var rateLimit int
	err := database.DB.Get(&rateLimit, "SELECT COALESCE(rate_limit, 0) FROM tenants WHERE id = ?", tenantID)
	if err != nil {
		log.Printf("RateLimit: failed to get rate_limit for tenant %s: %v", tenantID, err)
		if exists {
			return tl.limiter
		}
		return nil
	}

	limiterMu.Lock()
	defer limiterMu.Unlock()

	// 0 = 不限制
	if rateLimit <= 0 {
		delete(tenantLimiters, tenantID)
		return nil
	}

	// 检查是否需要更新
	if tl, exists := tenantLimiters[tenantID]; exists && tl.rateLimit == rateLimit {
		tl.updatedAt = time.Now()
		return tl.limiter
	}

	// 创建新的限流器（1分钟窗口）
	limiter := resilience.NewSlidingWindowLimiter(tenantID, rateLimit, 1*time.Minute)
	tenantLimiters[tenantID] = &tenantRateLimiter{
		limiter:   limiter,
		rateLimit: rateLimit,
		updatedAt: time.Now(),
	}
	return limiter
}

// ClearTenantLimiter 清除租户限流器（配置变更时调用）
func ClearTenantLimiter(tenantID string) {
	limiterMu.Lock()
	defer limiterMu.Unlock()
	delete(tenantLimiters, tenantID)
}
