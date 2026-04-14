package services

import (
	"fmt"
	"log"

	"vocalize/internal/config"

	"github.com/google/uuid"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

type EmailService struct {
	sendGridKey string
	sendEmails  bool
	frontendURL string
}

func NewEmailService(cfg *config.Config) *EmailService {
	return &EmailService{
		sendGridKey: cfg.SendGridAPIKey,
		sendEmails:  cfg.SendEmails,
		frontendURL: cfg.FrontendURL,
	}
}

// SendVerificationEmail sends email verification token
func (es *EmailService) SendVerificationEmail(userEmail, userID string) error {
	if !es.sendEmails {
		log.Printf("Email sending disabled. Would send verification email to: %s", userEmail)
		return nil
	}

	if es.sendGridKey == "" {
		return fmt.Errorf("SendGrid API key not configured")
	}

	// Create verification token (in production, store this in DB)
	token := uuid.New().String()

	// Create email
	from := mail.NewEmail("Vocalize", "noreply@vocalize.app")
	subject := "Verify Your Vocalize Account"
	to := mail.NewEmail("User", userEmail)

	// Build verification link
	verificationLink := fmt.Sprintf(
		"%s/verify-email?token=%s",
		es.frontendURL,
		token,
	)

	htmlContent := fmt.Sprintf(`
	<html>
	  <head></head>
	  <body>
		<h2>Welcome to Vocalize!</h2>
		<p>Thank you for signing up. Please verify your email address by clicking the link below:</p>
		<p>
		  <a href="%s" style="background-color: #2563eb; color: white; padding: 12px 24px; text-decoration: none; border-radius: 8px;">
			Verify Email
		  </a>
		</p>
		<p>Or copy and paste this link in your browser:</p>
		<p>%s</p>
		<p>This link will expire in 24 hours.</p>
		<p>Best regards,<br>The Vocalize Team</p>
	  </body>
	</html>
	`, verificationLink, verificationLink)

	message := mail.NewSingleEmail(from, subject, to, "Verify your email", htmlContent)

	client := sendgrid.NewSendClient(es.sendGridKey)
	response, err := client.Send(message)
	if err != nil {
		log.Printf("Failed to send verification email: %v", err)
		return err
	}

	log.Printf("Verification email sent. Status Code: %d", response.StatusCode)
	return nil
}

// SendPasswordResetEmail sends password reset token
func (es *EmailService) SendPasswordResetEmail(userEmail, resetToken string) error {
	if !es.sendEmails {
		log.Printf("Email sending disabled. Would send reset email to: %s", userEmail)
		return nil
	}

	if es.sendGridKey == "" {
		return fmt.Errorf("SendGrid API key not configured")
	}

	from := mail.NewEmail("Vocalize", "noreply@vocalize.app")
	subject := "Reset Your Vocalize Password"
	to := mail.NewEmail("User", userEmail)

	resetLink := fmt.Sprintf(
		"%s/reset-password?token=%s",
		es.frontendURL,
		resetToken,
	)

	htmlContent := fmt.Sprintf(`
	<html>
	  <head></head>
	  <body>
		<h2>Reset Your Password</h2>
		<p>We received a request to reset your password. Click the link below to set a new password:</p>
		<p>
		  <a href="%s" style="background-color: #2563eb; color: white; padding: 12px 24px; text-decoration: none; border-radius: 8px;">
			Reset Password
		  </a>
		</p>
		<p>This link will expire in 1 hour.</p>
		<p>If you didn't request this, please ignore this email.</p>
		<p>Best regards,<br>The Vocalize Team</p>
	  </body>
	</html>
	`, resetLink)

	message := mail.NewSingleEmail(from, subject, to, "Reset your password", htmlContent)

	client := sendgrid.NewSendClient(es.sendGridKey)
	response, err := client.Send(message)
	if err != nil {
		log.Printf("Failed to send password reset email: %v", err)
		return err
	}

	log.Printf("Password reset email sent. Status Code: %d", response.StatusCode)
	return nil
}
