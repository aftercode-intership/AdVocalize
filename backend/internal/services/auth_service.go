package services

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	//"os"
	"time"

	"vocalize/internal/config"
	"vocalize/internal/models"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"crypto/rand"
    "encoding/hex"
    
)

type AuthService struct {
	db             *sql.DB
	jwtSecret      string
	jwtExpiryHours int
	googleConfig   *oauth2.Config
	cfg            *config.Config
}

func NewAuthService(db *sql.DB, cfg *config.Config) *AuthService {
	googleConfig := &oauth2.Config{
		ClientID:     cfg.GoogleClientID,
		ClientSecret: cfg.GoogleClientSecret,
		RedirectURL:  cfg.GoogleRedirectURL,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
			"openid",
		},
		Endpoint: google.Endpoint,
	}

	return &AuthService{
		db:             db,
		jwtSecret:      cfg.JWTSecret,
		jwtExpiryHours: cfg.JWTExpiryHours,
		googleConfig:   googleConfig,
		cfg:            cfg,
	}
}

func (as *AuthService) GetGoogleConfig() *oauth2.Config {
//	return &oauth2.Config{
//		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
//		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
//		RedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
//		Scopes: []string{
//			"https://www.googleapis.com/auth/userinfo.email",
//			"https://www.googleapis.com/auth/userinfo.profile",
//			"openid",
//		},
//		Endpoint: google.Endpoint,
//	}
		return as.googleConfig
}


// Register creates a new user account
func (as *AuthService) Register(ctx context.Context, req *models.RegisterRequest) (*models.User, error) {
	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Check if email already exists
	var existingEmail string
	err = as.db.QueryRowContext(ctx, "SELECT email FROM users WHERE email = $1", req.Email).Scan(&existingEmail)
	if err == nil {
		return nil, fmt.Errorf("email already registered")
	}
	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("database error: %w", err)
	}

	// Create user
	user := &models.User{
		ID:               uuid.New().String(),
		Name:             req.Name,
		Email:            req.Email,
		PasswordHash:     string(hashedPassword),
		EmailVerified:    false,
		Language:         "en",
		SubscriptionTier: "FREE",
		CreditsRemaining: 50,
		AccountStatus:    "active",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	// Insert user
	_, err = as.db.ExecContext(
		ctx,
		`INSERT INTO users (id, name, email, password_hash, language, subscription_tier, credits_remaining, account_status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		user.ID, user.Name, user.Email, user.PasswordHash, user.Language, user.SubscriptionTier, user.CreditsRemaining, user.AccountStatus, user.CreatedAt, user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// Login authenticates a user with email and password
func (as *AuthService) Login(ctx context.Context, req *models.LoginRequest) (*models.User, string, error) {
	user := &models.User{}

	// Get user by email
	err := as.db.QueryRowContext(
		ctx,
		`SELECT id, name, email, password_hash, email_verified, email_verified_at, avatar_url, 
		 language, subscription_tier, credits_remaining, account_status, created_at, updated_at
		 FROM users WHERE email = $1 AND deleted_at IS NULL`,
		req.Email,
	).Scan(
		&user.ID, &user.Name, &user.Email, &user.PasswordHash, &user.EmailVerified, &user.EmailVerifiedAt,
		&user.Avatar, &user.Language, &user.SubscriptionTier, &user.CreditsRemaining, &user.AccountStatus, &user.CreatedAt, &user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, "", fmt.Errorf("invalid email or password")
	}
	if err != nil {
		return nil, "", fmt.Errorf("database error: %w", err)
	}

	// Check password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, "", fmt.Errorf("invalid email or password")
	}

	// Generate token
	token, err := config.GenerateToken(user.ID, user.Email, as.jwtSecret, as.jwtExpiryHours)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate token: %w", err)
	}

	// Create session
	session := &models.Session{
		ID:        uuid.New().String(),
		UserID:    user.ID,
		TokenHash: hashToken(token),
		ExpiresAt: time.Now().Add(time.Duration(as.jwtExpiryHours) * time.Hour),
		CreatedAt: time.Now(),
	}

	_, err = as.db.ExecContext(
		ctx,
		`INSERT INTO sessions (id, user_id, token_hash, expires_at, created_at, last_activity_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		session.ID, session.UserID, session.TokenHash, session.ExpiresAt, session.CreatedAt, session.CreatedAt,
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create session: %w", err)
	}

	return user, token, nil
}

// GoogleLogin or creates user from Google OAuth
func (as *AuthService) GoogleLogin(ctx context.Context, code string) (*models.User, string, error) {
	// Exchange code for token
	token, err := as.googleConfig.Exchange(ctx, code)
	if err != nil {
		return nil, "", fmt.Errorf("failed to exchange token: %w", err)
	}

	// Get user info
	resp, err := as.googleConfig.Client(ctx, token).Get("https://openidconnect.googleapis.com/v1/userinfo")
	if err != nil {
		return nil, "", fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	var googleUser models.GoogleOAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&googleUser); err != nil {
		return nil, "", fmt.Errorf("failed to decode user info: %w", err)
	}

	// Check if user exists by Google ID
	user := &models.User{}
	err = as.db.QueryRowContext(
		ctx,
		`SELECT id, name, email, email_verified, email_verified_at, avatar_url, 
		 language, subscription_tier, credits_remaining, account_status, created_at, updated_at
		 FROM users WHERE google_id = $1 AND deleted_at IS NULL`,
		googleUser.Sub,
	).Scan(
		&user.ID, &user.Name, &user.Email, &user.EmailVerified, &user.EmailVerifiedAt,
		&user.Avatar, &user.Language, &user.SubscriptionTier, &user.CreditsRemaining, &user.AccountStatus, &user.CreatedAt, &user.UpdatedAt,
	)

	// If user doesn't exist, create one
	if err == sql.ErrNoRows {
		user = &models.User{
			ID:               uuid.New().String(),
			Name:             googleUser.Name,
			Email:            googleUser.Email,
			GoogleID:         &googleUser.Sub,
			EmailVerified:    googleUser.EmailVerified,
			Language:         "en",
			SubscriptionTier: "FREE",
			CreditsRemaining: 50,
			AccountStatus:    "active",
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}

		// Store Google profile data
		profileData, _ := json.Marshal(googleUser)
		user.GoogleProfileData = (*json.RawMessage)(&profileData)

		if googleUser.EmailVerified {
			now := time.Now()
			user.EmailVerifiedAt = &now
		}

		// Insert user
		_, err = as.db.ExecContext(
			ctx,
			`INSERT INTO users (id, name, email, google_id, google_profile_data, email_verified, email_verified_at, 
			 language, subscription_tier, credits_remaining, account_status, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
			user.ID, user.Name, user.Email, user.GoogleID, user.GoogleProfileData, user.EmailVerified, user.EmailVerifiedAt,
			user.Language, user.SubscriptionTier, user.CreditsRemaining, user.AccountStatus, user.CreatedAt, user.UpdatedAt,
		)
		if err != nil {
			return nil, "", fmt.Errorf("failed to create user: %w", err)
		}
	} else if err != nil {
		return nil, "", fmt.Errorf("database error: %w", err)
	}

	// Generate JWT token
	jwtToken, err := config.GenerateToken(user.ID, user.Email, as.jwtSecret, as.jwtExpiryHours)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate token: %w", err)
	}

	// Create session
	session := &models.Session{
		ID:        uuid.New().String(),
		UserID:    user.ID,
		TokenHash: hashToken(jwtToken),
		ExpiresAt: time.Now().Add(time.Duration(as.jwtExpiryHours) * time.Hour),
		CreatedAt: time.Now(),
	}

	_, err = as.db.ExecContext(
		ctx,
		`INSERT INTO sessions (id, user_id, token_hash, expires_at, created_at, last_activity_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		session.ID, session.UserID, session.TokenHash, session.ExpiresAt, session.CreatedAt, session.CreatedAt,
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create session: %w", err)
	}

	return user, jwtToken, nil
}

// VerifyEmail marks email as verified
func (as *AuthService) VerifyEmail(ctx context.Context, token string) error {
	var userID string
	var usedAt *time.Time

	err := as.db.QueryRowContext(
		ctx,
		`SELECT user_id, used_at FROM email_verification_tokens WHERE token = $1 AND expires_at > NOW()`,
		token,
	).Scan(&userID, &usedAt)

	if err == sql.ErrNoRows {
		return fmt.Errorf("invalid or expired verification token")
	}
	if err != nil {
		return fmt.Errorf("database error: %w", err)
	}

	if usedAt != nil {
		return fmt.Errorf("token already used")
	}

	// Update user
	_, err = as.db.ExecContext(
		ctx,
		`UPDATE users SET email_verified = true, email_verified_at = NOW(), updated_at = NOW() WHERE id = $1`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("failed to verify email: %w", err)
	}

	// Mark token as used
	_, err = as.db.ExecContext(
		ctx,
		`UPDATE email_verification_tokens SET used_at = NOW() WHERE token = $1`,
		token,
	)
	if err != nil {
		return fmt.Errorf("failed to update token: %w", err)
	}

	return nil
}

// GetUser retrieves user by ID
func (as *AuthService) GetUser(ctx context.Context, userID string) (*models.User, error) {
	user := &models.User{}

	err := as.db.QueryRowContext(
		ctx,
		`SELECT id, name, email, email_verified, email_verified_at, avatar_url, 
		 language, subscription_tier, credits_remaining, account_status, created_at, updated_at
		 FROM users WHERE id = $1 AND deleted_at IS NULL`,
		userID,
	).Scan(
		&user.ID, &user.Name, &user.Email, &user.EmailVerified, &user.EmailVerifiedAt,
		&user.Avatar, &user.Language, &user.SubscriptionTier, &user.CreditsRemaining, &user.AccountStatus, &user.CreatedAt, &user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	return user, nil
}

// Helper function
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", hash)
}

func (as *AuthService) GeneratePasswordResetToken(ctx context.Context, email string) (string, error) {
	// Check the email exists first
	var userID string
	err := as.db.QueryRowContext(ctx,
		`SELECT id FROM users WHERE email = $1 AND deleted_at IS NULL`,
		email,
	).Scan(&userID)
 
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("email not found")
	}
	if err != nil {
		return "", fmt.Errorf("database error: %w", err)
	}
 
	// Generate a cryptographically secure random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)
 
	// Store the hash (not the raw token) for security
	tokenHash := hashToken(token)
 
	// Expire any existing unused tokens for this user before creating a new one
	// This prevents token accumulation and ensures only the latest token works
	as.db.ExecContext(ctx,
		`UPDATE password_reset_tokens
		 SET used_at = NOW()
		 WHERE user_id = $1 AND used_at IS NULL`,
		userID,
	)
 
	// Insert the new token
	_, err = as.db.ExecContext(ctx,
		`INSERT INTO password_reset_tokens (id, user_id, token_hash, expires_at, created_at)
		 VALUES ($1, $2, $3, NOW() + INTERVAL '1 hour', NOW())`,
		uuid.New().String(), userID, tokenHash,
	)
	if err != nil {
		return "", fmt.Errorf("failed to store reset token: %w", err)
	}
 
	return token, nil
}
 func (as *AuthService) StoreEmailVerificationToken(ctx context.Context, userID, email string) (string, error) {
	// Generate random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)
 
	// Store in DB (expires in 24 hours)
	_, err := as.db.ExecContext(ctx, `
		INSERT INTO email_verification_tokens (id, user_id, token, expires_at, created_at)
		VALUES (gen_random_uuid(), $1, $2, NOW() + INTERVAL '24 hours', NOW())
	`, userID, token)
	if err != nil {
		return "", fmt.Errorf("failed to store token: %w", err)
	}
 
	return token, nil
}
// ResetPassword handles the actual password reset using the token
func (as *AuthService) ResetPassword(ctx context.Context, token, newPassword string) error {
	tokenHash := hashToken(token)
 
	// Look up the token
	var userID string
	var usedAt *time.Time
 
	err := as.db.QueryRowContext(ctx,
		`SELECT user_id, used_at
		 FROM password_reset_tokens
		 WHERE token_hash = $1 AND expires_at > NOW()`,
		tokenHash,
	).Scan(&userID, &usedAt)
 
	if err == sql.ErrNoRows {
		return fmt.Errorf("invalid or expired reset link. Please request a new one.")
	}
	if err != nil {
		return fmt.Errorf("database error: %w", err)
	}
 
	if usedAt != nil {
		return fmt.Errorf("this reset link has already been used. Please request a new one.")
	}
 
	// Hash the new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to process new password: %w", err)
	}
 
	// Do the update in a transaction so both steps succeed or both fail
	tx, err := as.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("transaction failed: %w", err)
	}
	defer tx.Rollback()
 
	_, err = tx.ExecContext(ctx,
		`UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2`,
		string(hashedPassword), userID,
	)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}
 
	_, err = tx.ExecContext(ctx,
		`UPDATE password_reset_tokens SET used_at = NOW() WHERE token_hash = $1`,
		tokenHash,
	)
	if err != nil {
		return fmt.Errorf("failed to invalidate token: %w", err)
	}
 
	return tx.Commit()
}