package middleware

import (
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/ratelimit"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// 认证端点速率限制配置
// 环境变量可覆盖默认值，设置为 0 禁用限制
var (
	authRateLimitPerMinute int
	authRateLimitPerHour   int
)

func init() {
	authRateLimitPerMinute = getEnvInt("AUTH_RATE_LIMIT_PER_MINUTE", 10)
	authRateLimitPerHour = getEnvInt("AUTH_RATE_LIMIT_PER_HOUR", 60)
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if n, err := strconv.Atoi(val); err == nil && n >= 0 {
			return n
		}
	}
	return defaultVal
}

// AuthRateLimit 返回认证端点的 IP 级别速率限制中间件
// 防护：暴力破解密码、撞库攻击、枚举用户
// 使用滑动窗口算法，支持 Redis 分布式部署和本地内存回退
func AuthRateLimit(redisClient *redis.Client) gin.HandlerFunc {
	// 如果两个限制都禁用了，返回空中间件
	if authRateLimitPerMinute <= 0 && authRateLimitPerHour <= 0 {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	minuteLimiter := ratelimit.New(redisClient, "auth:ratelimit:minute:", time.Minute, "")
	hourLimiter := ratelimit.New(redisClient, "auth:ratelimit:hour:", time.Hour, "")

	return func(c *gin.Context) {
		ip := c.ClientIP()

		// 检查每分钟限制
		if authRateLimitPerMinute > 0 {
			if !minuteLimiter.Allow(c.Request.Context(), ip, authRateLimitPerMinute) {
				logger.Warnf(c.Request.Context(),
					"[RateLimit] 认证端点每分钟限制超限: ip=%s path=%s",
					ip, c.Request.URL.Path)
				c.JSON(http.StatusTooManyRequests, gin.H{
					"error":   "Rate limit exceeded",
					"message": "Too many requests. Please try again later.",
				})
				c.Abort()
				return
			}
		}

		// 检查每小时限制
		if authRateLimitPerHour > 0 {
			if !hourLimiter.Allow(c.Request.Context(), ip, authRateLimitPerHour) {
				logger.Warnf(c.Request.Context(),
					"[RateLimit] 认证端点每小时限制超限: ip=%s path=%s",
					ip, c.Request.URL.Path)
				c.JSON(http.StatusTooManyRequests, gin.H{
					"error":   "Rate limit exceeded",
					"message": "Too many requests. Please try again later.",
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// LoginRateLimit 返回登录端点更严格的速率限制中间件
// 防护：暴力破解密码（比通用认证限制更严格）
func LoginRateLimit(redisClient *redis.Client) gin.HandlerFunc {
	// 登录限制：每分钟 5 次，每小时 20 次
	minuteLimiter := ratelimit.New(redisClient, "login:ratelimit:minute:", time.Minute, "")
	hourLimiter := ratelimit.New(redisClient, "login:ratelimit:hour:", time.Hour, "")

	return func(c *gin.Context) {
		ip := c.ClientIP()

		if !minuteLimiter.Allow(c.Request.Context(), ip, 5) {
			logger.Warnf(c.Request.Context(),
				"[RateLimit] 登录端点每分钟限制超限: ip=%s", ip)
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "Rate limit exceeded",
				"message": "Too many login attempts. Please try again in a minute.",
			})
			c.Abort()
			return
		}

		if !hourLimiter.Allow(c.Request.Context(), ip, 20) {
			logger.Warnf(c.Request.Context(),
				"[RateLimit] 登录端点每小时限制超限: ip=%s", ip)
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "Rate limit exceeded",
				"message": "Too many login attempts. Please try again later.",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
