// backend/main.go
package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"vocalize/internal/config"
	"vocalize/internal/database"
	"vocalize/internal/handlers"
	"vocalize/internal/middleware"
	"vocalize/internal/services"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
)

// Handlers groups all route handlers so they can be passed around together.
type Handlers struct {
	Auth        *handlers.AuthHandler
	Chat        *handlers.ChatHandler
	MessageEdit *handlers.MessageEditHandler
	Escalation  *handlers.EscalationHandler
	Profile     *handlers.ProfileHandler
	Script      *handlers.ScriptHandler
}

func main() {
	app := fiber.New(fiber.Config{
		// ✅ FIXED: ErrorHandler signature uses fiber.Ctx (interface), not *fiber.Ctx (pointer)
		ErrorHandler: func(c fiber.Ctx, err error) error {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		},
	})
	app.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000"},
		AllowCredentials: true,
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		MaxAge:           86400, // cache preflight for 24h
	}))

	// app.Use(fiber.Logger()) // Logging disabled due to compiler error

	// --- Config & Database ---

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	db, err := database.NewFromURL(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	dbConn := db.GetConnection()

	// --- Services ---

	userService := services.NewUserService(dbConn)
	profileService := services.NewProfileService(dbConn)
	creditService := services.NewCreditService(dbConn)
	emailService := services.NewEmailService(cfg)
	authService := services.NewAuthService(dbConn, cfg)

	// Chat services
	pcm := services.NewPromptContextManager(dbConn)

	glmKey := os.Getenv("GLM_API_KEY")
	if glmKey == "" {
		log.Fatal("GLM_API_KEY environment variable is required")
	}

	glmService := services.NewGLMService(glmKey, pcm)
	chatService := services.NewChatService(dbConn, glmService, creditService, pcm)
	messageEditService := services.NewMessageEditService(dbConn, chatService)
	supportBotService := services.NewSupportBotService(chatService, dbConn)

	scriptGenURL := os.Getenv("SCRIPT_GEN_SERVICE_URL")
	if scriptGenURL == "" {
		scriptGenURL = "http://localhost:8001"
	}
	scriptService := services.NewScriptService(dbConn, scriptGenURL, creditService)

	// --- Handlers ---

	authHandler := handlers.NewAuthHandler(authService, emailService)
	profileHandler := handlers.NewProfileHandler(profileService)

	wsHandler := handlers.NewWebSocketHandler(chatService)
	chatHandler := handlers.NewChatHandler(chatService, wsHandler)
	messageEditHandler := handlers.NewMessageEditHandler(messageEditService)
	escalationHandler := handlers.NewEscalationHandler(supportBotService)
	scriptHandler := handlers.NewScriptHandler(scriptService)

	h := &Handlers{
		Auth:        authHandler,
		Chat:        chatHandler,
		MessageEdit: messageEditHandler,
		Escalation:  escalationHandler,
		Profile:     profileHandler,
		Script:      scriptHandler,
	}
	// --- Middleware & Routes ---

	rbacMiddleware := middleware.NewRBACMiddleware(userService)

	// ✅ FIXED: use cfg.JWTSecret — jwtSecret was never declared before
	setupRoutes(app, authHandler, h, cfg, rbacMiddleware.RequireRole, cfg.JWTSecret)

	fmt.Printf("🚀 Vocalize backend running on http://localhost:%s\n", cfg.Port)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
	log.Println(strings.Repeat("=", 60))
	log.Println("🔍 DEBUGGING DATABASE SCHEMA")
	log.Println(strings.Repeat("=", 60))

	if err := chatService.TestDatabaseConnection(); err != nil {
		log.Printf("❌ Database connection test failed: %v", err)
	}

	// Check critical tables
	tables := []string{"chat_sessions", "chat_messages", "users", "sessions"}
	for _, table := range tables {
		exists, err := chatService.TableExists(table)
		if !exists {
			log.Printf("❌ CRITICAL: Table '%s' not found in database!", table)
		}
		if err != nil {
			log.Printf("❌ Error checking table '%s': %v", table, err)
		}

		// If table exists, show its schema
		if exists {
			if err := chatService.GetTableSchema(table); err != nil {
				log.Printf("Warning: Could not get schema for %s: %v", table, err)
			}
		}
	}

	log.Println(strings.Repeat("=", 60))
	log.Println("Starting server...")
	log.Println(strings.Repeat("=", 60))

}

func setupRoutes(
	app *fiber.App,
	authHandler *handlers.AuthHandler,
	h *Handlers,
	cfg *config.Config,
	requireRole func(string) fiber.Handler,
	jwtSecret string, // ✅ received as parameter — no undefined variable
) {
	// --- Public auth routes (no JWT required) ---
	auth := app.Group("/api/auth")
	auth.Post("/register", authHandler.Register)
	auth.Post("/login", authHandler.Login)
	auth.Get("/google/login", authHandler.GoogleLogin)
	auth.Get("/google/callback", authHandler.GoogleCallback)
	auth.Get("/verify", authHandler.VerifyEmail)
	auth.Post("/logout", authHandler.Logout)
	auth.Post("/forgot-password", h.Auth.ForgotPassword) // NEW
	auth.Post("/reset-password", h.Auth.ResetPassword)

	// --- Protected routes (JWT required) ---
	protected := app.Group("/api", middleware.AuthMiddleware(jwtSecret))
	protected.Get("/auth/me", authHandler.GetCurrentUser)

	// Profile routes
	protected.Get("/profile", h.Profile.GetProfile)
	protected.Patch("/profile", h.Profile.UpdateProfile)

	// Script routes
	protected.Post("/generate/script", h.Script.GenerateScript)
	protected.Get("/generate/script/:id", h.Script.GetScript)
	protected.Post("/chat/enhance", h.Chat.EnhancePrompt)

	// Chat + WebSocket routes
	setupChatRoutes(app, h, jwtSecret, requireRole)

	// Health check (public)
	app.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "vocalize-backend"})
	})
}

func setupChatRoutes(
	app *fiber.App,
	h *Handlers,
	jwtSecret string,
	requireRole func(string) fiber.Handler,
) {
	// All chat routes require authentication
	chat := app.Group("/api/chat", middleware.AuthMiddleware(jwtSecret))

	chat.Get("/sessions", h.Chat.ListSessions)
	chat.Get("/library/history", h.Chat.GetLibraryHistory)
	chat.Post("/sessions", h.Chat.CreateSession)
	chat.Get("/sessions/:id/history", h.Chat.GetHistory)
	chat.Post("/sessions/:id/message", h.Chat.SendMessage)
	chat.Delete("/sessions/:id", h.Chat.ArchiveSession)
	chat.Patch("/messages/:id", h.MessageEdit.EditMessage)
	chat.Post("/messages/:id/regenerate", h.MessageEdit.RegenerateResponse)
	chat.Get("/messages/:id/versions", h.MessageEdit.GetVersions)

	app.Post("/api/chat/enhance",
		middleware.AuthMiddleware(jwtSecret),
		h.Chat.EnhancePrompt,
	)
	app.Get("/api/chat/sessions/:id/ws",
		middleware.AuthMiddleware(jwtSecret), // runs first as HTTP middleware
		h.Chat.WebSocketHandler(),            // websocket.New(...) takes over after
	)

	// Support escalation
	support := app.Group("/api/support", middleware.AuthMiddleware(jwtSecret))
	support.Post("/escalate", h.Escalation.CreateEscalation)

	// Admin support (requires admin role on top of auth)
	admin := app.Group("/api/admin/support",
		middleware.AuthMiddleware(jwtSecret),
		requireRole("admin"),
	)
	admin.Get("/escalations", h.Escalation.GetPendingEscalations)
	admin.Post("/escalations/:id/assign", h.Escalation.AssignEscalation)
	admin.Post("/escalations/:id/close", h.Escalation.CloseEscalation)
}
