package handlers

import (
	"context"
	"net/http"
	"os"
	"time"

	"vocalize/internal/models"
	"vocalize/internal/services"

	"github.com/gofiber/fiber/v3"
	"golang.org/x/oauth2"
	
)

type AuthHandler struct {
	authService  *services.AuthService
	emailService *services.EmailService
}

func NewAuthHandler(authService *services.AuthService, emailService *services.EmailService) *AuthHandler {
	return &AuthHandler{
		authService:  authService,
		emailService: emailService,
	}
}

// Register handles POST /api/auth/register
func (h *AuthHandler) Register(c fiber.Ctx) error {
	var req models.RegisterRequest

	if err := c.Bind().Body(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Basic validation
	if len(req.Name) < 2 {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Name must be at least 2 characters",
		})
	}

	if len(req.Password) < 8 {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Password must be at least 8 characters",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	user, err := h.authService.Register(ctx, &req)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Send verification email (async)
	go h.emailService.SendVerificationEmail(user.Email, user.ID)

	return c.Status(http.StatusCreated).JSON(fiber.Map{
		"message": "User registered successfully. Please check your email to verify your account.",
		"user": fiber.Map{
			"id":    user.ID,
			"name":  user.Name,
			"email": user.Email,
		},
	})
}

// Login handles POST /api/auth/login
func (h *AuthHandler) Login(c fiber.Ctx) error {
	var req models.LoginRequest

	if err := c.Bind().Body(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	user, token, err := h.authService.Login(ctx, &req)
	if err != nil {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid email or password",
		})
	}

	// Set httpOnly cookie - Secure flag should be true in production
	isProduction := os.Getenv("ENVIRONMENT") == "production"
	c.Cookie(&fiber.Cookie{
		Name:     "auth_token",
		Value:    token,
		Expires:  time.Now().Add(7 * 24 * time.Hour),
		HTTPOnly: true,
		Secure:   isProduction,
		SameSite: "Lax",
		Path:     "/",
	})

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"token":      token,
		"expires_in": 24 * 60 * 60, // 24 hours in seconds
		"user": fiber.Map{
			"id":                user.ID,
			"name":              user.Name,
			"email":             user.Email,
			"email_verified":    user.EmailVerified,
			"subscription_tier": user.SubscriptionTier,
			"credits_remaining": user.CreditsRemaining,
		},
	})
}

// GoogleLogin handles GET /api/auth/google/login
func (h *AuthHandler) GoogleLogin(c fiber.Ctx) error {
	// Redirect to Google OAuth consent screen
	googleConfig := h.authService.GetGoogleConfig()
	authURL := googleConfig.AuthCodeURL("state", oauth2.AccessTypeOffline)
	c.Set("Location", authURL)
	return c.SendStatus(fiber.StatusTemporaryRedirect)
}

// GoogleCallback handles GET /api/auth/google/callback
func (h *AuthHandler) GoogleCallback(c fiber.Ctx) error {
	code := c.Query("code")
 
	if code == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Authorization code not provided",
		})
	}
 
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
 
	_, token, err := h.authService.GoogleLogin(ctx, code)
	if err != nil {
		// Redirect to login with error instead of returning JSON,
		// since the user's browser is making this request
		frontendURL := os.Getenv("FRONTEND_URL")
		if frontendURL == "" {
			frontendURL = "http://localhost:3000"
		}
		c.Set("Location", frontendURL+"/login?error=google_auth_failed")
		return c.SendStatus(fiber.StatusTemporaryRedirect)
	}
 
	isProduction := os.Getenv("ENVIRONMENT") == "production"
	c.Cookie(&fiber.Cookie{
		Name:     "auth_token",
		Value:    token,
		Expires:  time.Now().Add(7 * 24 * time.Hour),
		HTTPOnly: true,
		Secure:   isProduction,
		SameSite: "Lax",
		Path:     "/",
	})
 
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}
	redirectURL := frontendURL + "/?auth=success"
	c.Set("Location", redirectURL)
	return c.SendStatus(fiber.StatusTemporaryRedirect)
}

// VerifyEmail handles GET /api/auth/verify?token=...
func (h *AuthHandler) VerifyEmail(c fiber.Ctx) error {
	token := c.Query("token")
	if token == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Verification token not provided",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := h.authService.VerifyEmail(ctx, token)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"message": "Email verified successfully",
	})
}

// GetCurrentUser handles GET /api/auth/me
func (h *AuthHandler) GetCurrentUser(c fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	user, err := h.authService.GetUser(ctx, userID)
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{
			"error": "User not found",
		})
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"user": fiber.Map{
			"id":                user.ID,
			"name":              user.Name,
			"email":             user.Email,
			"email_verified":    user.EmailVerified,
			"avatar":            user.Avatar,
			"language":          user.Language,
			"subscription_tier": user.SubscriptionTier,
			"credits_remaining": user.CreditsRemaining,
		},
	})
}

// Logout handles POST /api/auth/logout
func (h *AuthHandler) Logout(c fiber.Ctx) error {
	c.ClearCookie("auth_token")
	return c.Status(http.StatusOK).JSON(fiber.Map{
		"message": "Logged out successfully",
	})
}

func (h *AuthHandler) ForgotPassword(c fiber.Ctx) error {
	var req struct {
		Email string `json:"email"`
	}
 
	if err := c.Bind().Body(&req); err != nil || req.Email == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Email is required",
		})
	}
 
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
 
	// Generate token and send email asynchronously so the response is fast.
	// If the email doesn't exist, GeneratePasswordResetToken returns an error
	// which we silently ignore — the user gets the same 200 response either way.
	go func() {
		token, err := h.authService.GeneratePasswordResetToken(ctx, req.Email)
		if err != nil {
			return // email doesn't exist or DB error — silently ignore
		}
		h.emailService.SendPasswordResetEmail(req.Email, token)
	}()
 
	return c.Status(http.StatusOK).JSON(fiber.Map{
		"message": "If an account with that email exists, a password reset link has been sent.",
	})
}
 
// ResetPassword handles POST /api/auth/reset-password
//
// The user sends the token from the email link plus their new password.
// We validate the token (must exist, not expired, not used), then update
// the user's password and mark the token as used.
func (h *AuthHandler) ResetPassword(c fiber.Ctx) error {
	var req struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
 
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}
 
	if req.Token == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Reset token is required",
		})
	}
 
	if len(req.NewPassword) < 8 {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Password must be at least 8 characters",
		})
	}
 
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
 
	if err := h.authService.ResetPassword(ctx, req.Token, req.NewPassword); err != nil {
		// We do expose the error here since the user needs actionable feedback
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
 
	return c.Status(http.StatusOK).JSON(fiber.Map{
		"message": "Password reset successfully. You can now log in with your new password.",
	})
}