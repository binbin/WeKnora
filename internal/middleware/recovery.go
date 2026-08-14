package middleware

import (
	"fmt"
	"runtime/debug"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// Recovery is a middleware that recovers from panics
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Get request ID from context
				ctx := c.Request.Context()
				requestID, _ := c.Get("RequestID")

				// Print stacktrace
				stacktrace := debug.Stack()
				// Log error with structured logger
				logger.ErrorWithFields(ctx, fmt.Errorf("panic: %v", err), logrus.Fields{
					"request_id": requestID,
					"stacktrace": string(stacktrace),
				})

				// 返回500错误（生产环境隐藏内部错误详情）
				message := "An unexpected error occurred"
				if gin.Mode() != gin.ReleaseMode {
					message = fmt.Sprintf("%v", err)
				}
				c.AbortWithStatusJSON(500, gin.H{
					"error":   "Internal Server Error",
					"message": message,
				})
			}
		}()

		c.Next()
	}
}
