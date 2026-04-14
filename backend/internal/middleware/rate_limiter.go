// backend/internal/middleware/rate_limiter.go
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/redis/go-redis/v9"
)

type LoginRateLimiter struct {
	redis           *redis.Client
	maxAttempts     int
	lockoutDuration time.Duration
}

func NewLoginRateLimiter(redis *redis.Client) *LoginRateLimiter {
	return &LoginRateLimiter{
		redis:           redis,
		maxAttempts:     5,
		lockoutDuration: 15 * time.Minute,
	}
}

// IsLocked checks if user is rate limited
func (lr *LoginRateLimiter) IsLocked(email string) (bool, error) {
	key := fmt.Sprintf("login_locked:%s", email)

	val, err := lr.redis.Get(context.Background(), key).Result()
	if err == redis.Nil {
		return false, nil // Key doesn't exist, not locked
	}

	if err != nil {
		return false, err
	}

	return val == "locked", nil
}

// RecordFailure records a failed login attempt
func (lr *LoginRateLimiter) RecordFailure(email string) error {
	key := fmt.Sprintf("login_attempts:%s", email)

	// Increment attempts
	val, err := lr.redis.Incr(context.Background(), key).Result()
	if err != nil {
		return err
	}

	// Set expiry on first attempt (5-minute window)
	if val == 1 {
		lr.redis.Expire(context.Background(), key, 5*time.Minute)
	}

	// Lock account if max attempts exceeded
	if val >= int64(lr.maxAttempts) {
		lockKey := fmt.Sprintf("login_locked:%s", email)
		lr.redis.Set(context.Background(), lockKey, "locked", lr.lockoutDuration)
	}

	return nil
}

// ResetFailures clears failed attempts
func (lr *LoginRateLimiter) ResetFailures(email string) error {
	key := fmt.Sprintf("login_attempts:%s", email)
	return lr.redis.Del(context.Background(), key).Err()
}

// Middleware
func (lr *LoginRateLimiter) Middleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		email := c.FormValue("email")
		if email == "" {
			return c.Status(http.StatusBadRequest).JSON(map[string]string{
				"error": "Email required",
			})
		}

		locked, err := lr.IsLocked(email)
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(map[string]string{
				"error": "Server error",
			})
		}

		if locked {
			return c.Status(http.StatusTooManyRequests).JSON(map[string]string{
				"error": "Too many failed attempts. Try again in 15 minutes.",
			})
		}

		// Store email in context for handler
		c.Locals("email", email)
		return c.Next()
	}
}
