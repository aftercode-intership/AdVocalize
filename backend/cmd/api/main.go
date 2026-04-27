// backend/cmd/api/main.go
package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"vocalize/internal/config"
	"vocalize/internal/database"
	"vocalize/internal/handlers"
	"vocalize/internal/middleware"
	"vocalize/internal/services"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
)

type Handlers struct {
	Auth        *handlers.AuthHandler
	Chat        *handlers.ChatHandler
	MessageEdit *handlers.MessageEditHandler
	Escalation  *handlers.EscalationHandler
	Profile     *handlers.ProfileHandler
	Script      *handlers.ScriptHandler
	TTS         *handlers.TTSHandler
	Dashboard   *handlers.DashboardHandler
}

func main() {
	// ===== DETAILED LOGGING =====
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println(strings.Repeat("=", 70))
	log.Println("VOCALIZE BACKEND STARTUP")
	log.Println(strings.Repeat("=", 70))

	app := fiber.New(fiber.Config{
		ErrorHandler: func(c fiber.Ctx, err error) error {
			// Log the actual error
			log.Printf("[ERROR] Handler error: %v", err)
			log.Printf("[ERROR] Request path: %s %s", c.Method(), c.Path())
			log.Printf("[ERROR] User ID: %v", c.Locals("user_id"))

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
		MaxAge:           86400,
	}))

	// ===== CONFIG & DATABASE =====
	log.Println("[STARTUP] Loading configuration...")
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("[FATAL] Failed to load config: %v", err)
	}

	log.Printf("[STARTUP] Database URL: %s (port: %s)", cfg.DatabaseURL, cfg.Port)
	log.Println("[STARTUP] Connecting to database...")

	db, err := database.NewFromURL(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("[FATAL] Failed to connect to database: %v", err)
	}
	defer db.Close()

	dbConn := db.GetConnection()
	log.Println("[STARTUP] ✓ Database connected successfully")

	// ===== DATABASE VERIFICATION =====
	log.Println("[STARTUP] Verifying database schema...")
	if err := verifyDatabaseSchema(dbConn); err != nil {
		log.Printf("[WARNING] Database schema verification failed: %v", err)
	}

	// ===== SERVICES =====
	log.Println("[STARTUP] Initializing services...")
	userService := services.NewUserService(dbConn)
	profileService := services.NewProfileService(dbConn)
	creditService := services.NewCreditService(dbConn)
	emailService := services.NewEmailService(cfg)

	redisClient := config.GetRedisClient(cfg)
	_ = middleware.NewLoginRateLimiter(redisClient) // Ready for future use

	authService := services.NewAuthService(dbConn, cfg)

	ttsService := services.NewTTSService(dbConn, cfg.TTSServiceURL, creditService)

	pcm := services.NewPromptContextManager(dbConn)
	dashboardService   := services.NewDashboardService(dbConn)

	nvidiaKey := cfg.NVIDIAAPIKey
	glmKey := cfg.GLMAPIKey
	glmProvider := cfg.GLMProvider
	glmForceNvidia := cfg.GLMForceNVIDIA

	log.Printf("[STARTUP] GLM config - NVIDIA key: %t, OpenRouter key: %t, Provider: %q, Force NVIDIA: %t",
		nvidiaKey != "", glmKey != "", glmProvider, glmForceNvidia)

	// Determine provider with sensible defaults:
	// Explicit flags take precedence, then default to NVIDIA if its key is present.
	switch {
	case glmForceNvidia || glmProvider == "nvidia":
		if nvidiaKey == "" {
			if cfg.Environment == "development" {
				log.Println("[STARTUP] Warning: NVIDIA_API_KEY not set, using mock GLM service for development")
				glmProvider = "mock"
			} else {
				log.Fatal("[FATAL] NVIDIA_API_KEY required when GLM_PROVIDER=nvidia or GLM_FORCE_NVIDIA=true")
			}
		} else {
			glmProvider = "nvidia"
			if glmKey != "" {
				log.Println("[STARTUP] Using NVIDIA GLM (with retries + OpenRouter fallback)")
			} else {
				log.Println("[STARTUP] Using NVIDIA GLM (no OpenRouter fallback — set GLM_API_KEY to enable)")
			}
		}
	case glmProvider == "openrouter":
		if glmKey == "" {
			if cfg.Environment == "development" {
				log.Println("[STARTUP] Warning: GLM_API_KEY not set, using mock GLM service for development")
				glmProvider = "mock"
			} else {
				log.Fatal("[FATAL] GLM_API_KEY required for OpenRouter")
			}
		} else {
			glmProvider = "openrouter"
			log.Println("[STARTUP] Using OpenRouter GLM")
		}
	case nvidiaKey != "":
		// Default to NVIDIA when NVIDIA_API_KEY is present and no explicit provider chosen
		glmProvider = "nvidia"
		if glmKey != "" {
			log.Println("[STARTUP] Using NVIDIA GLM (with retries + OpenRouter fallback)")
		} else {
			log.Println("[STARTUP] Using NVIDIA GLM (no OpenRouter fallback — set GLM_API_KEY to enable)")
		}
	case glmKey != "":
		// Default to OpenRouter when only GLM_API_KEY is present
		glmProvider = "openrouter"
		log.Println("[STARTUP] Using OpenRouter GLM")
	default:
		if cfg.Environment == "development" {
			log.Println("[STARTUP] Warning: No GLM API keys configured, using mock GLM service for development")
			glmProvider = "mock"
		} else {
			log.Fatal("[FATAL] Either NVIDIA_API_KEY or GLM_API_KEY must be set")
		}
	}

	glmService := services.NewGLMService(nvidiaKey, glmKey, glmProvider, cfg.Environment, pcm)
	chatService := services.NewChatService(dbConn, glmService, creditService, pcm)

	// Skip startup GLM test: it blocks for 60s+ when NVIDIA is slow,
	// and the first real request will trigger the same fallback logic.
	log.Println("[STARTUP] GLM service initialized (runtime fallback enabled)")

	// ===== CRITICAL: Test chat service =====
	log.Println("[STARTUP] Testing chat service database access...")
	if err := testChatService(chatService); err != nil {
		log.Printf("[WARNING] Chat service test failed: %v", err)
	}

	messageEditService := services.NewMessageEditService(dbConn, chatService)
	supportBotService := services.NewSupportBotService(chatService, dbConn)

	scriptService := services.NewScriptService(dbConn, cfg.ScriptGenServiceURL, creditService)

	log.Println("[STARTUP] ✓ All services initialized")

	// ===== HANDLERS =====
	log.Println("[STARTUP] Creating handlers...")
	authHandler := handlers.NewAuthHandler(authService, emailService)
	profileHandler := handlers.NewProfileHandler(profileService)
	wsHandler := handlers.NewWebSocketHandler(chatService)
	chatHandler := handlers.NewChatHandler(chatService, wsHandler)
	messageEditHandler := handlers.NewMessageEditHandler(messageEditService)
	escalationHandler := handlers.NewEscalationHandler(supportBotService)
	scriptHandler := handlers.NewScriptHandler(scriptService)
	ttsHandler := handlers.NewTTSHandler(ttsService)
	dashboardHandler   := handlers.NewDashboardHandler(dashboardService)

	h := &Handlers{
		Auth:        authHandler,
		Chat:        chatHandler,
		MessageEdit: messageEditHandler,
		Escalation:  escalationHandler,
		Profile:     profileHandler,
		Script:      scriptHandler,
		TTS:         ttsHandler,
		Dashboard:   dashboardHandler,
	}

	rbacMiddleware := middleware.NewRBACMiddleware(userService)

	log.Println("[STARTUP] ✓ Handlers created")
	log.Println("[STARTUP] Setting up routes...")
	setupRoutes(app, authHandler, h, cfg, rbacMiddleware.RequireRole, cfg.JWTSecret)
	log.Println("[STARTUP] ✓ Routes configured")

	// ===== START SERVER =====
	log.Println(strings.Repeat("=", 70))
	fmt.Printf("🚀 Vocalize backend running on http://localhost:%s\n", cfg.Port)
	fmt.Println("   API:     http://localhost:" + cfg.Port + "/api")
	fmt.Println("   Health:  http://localhost:" + cfg.Port + "/health")
	log.Println(strings.Repeat("=", 70))

	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatalf("[FATAL] Failed to start server: %v", err)
	}
}

// ===== VERIFICATION FUNCTIONS =====

func verifyDatabaseSchema(db *sql.DB) error {
	log.Println("[VERIFY] Checking if required tables exist...")

	tables := []string{"users", "chat_sessions", "chat_messages"}

	for _, tableName := range tables {
		exists, err := tableExists(db, tableName)
		if err != nil {
			log.Printf("[VERIFY] Error checking table %s: %v", tableName, err)
			continue
		}

		if !exists {
			log.Printf("[VERIFY] ✗ MISSING TABLE: %s", tableName)
		} else {
			log.Printf("[VERIFY] ✓ Table exists: %s", tableName)
			if err := printTableSchema(db, tableName); err != nil {
				log.Printf("[VERIFY] Error printing schema for %s: %v", tableName, err)
			}
		}
	}

	return nil
}

func tableExists(db *sql.DB, tableName string) (bool, error) {
	var exists bool
	err := db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = $1
		)
	`, tableName).Scan(&exists)
	return exists, err
}

func printTableSchema(db *sql.DB, tableName string) error {
	rows, err := db.Query(`
		SELECT column_name, data_type, is_nullable
		FROM information_schema.columns
		WHERE table_schema = 'public'
		AND table_name = $1
		ORDER BY ordinal_position
	`, tableName)
	if err != nil {
		return err
	}
	defer rows.Close()

	log.Printf("[VERIFY]   Columns in %s:", tableName)
	for rows.Next() {
		var colName, dataType, nullable string
		if err := rows.Scan(&colName, &dataType, &nullable); err != nil {
			return err
		}
		log.Printf("[VERIFY]     - %s (%s, nullable: %s)", colName, dataType, nullable)
	}

	return nil
}

func testChatService(cs *services.ChatService) error {
	log.Println("[TEST] Attempting to query chat_sessions table...")

	// This will test if the table exists and is accessible
	sessions, err := cs.ListSessions("00000000-0000-0000-0000-000000000000", 1)
	if err != nil {
		log.Printf("[TEST] ✗ ListSessions failed: %v", err)
		return err
	}

	log.Printf("[TEST] ✓ ListSessions succeeded (found %d sessions)", len(sessions))
	return nil
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

	protected.Post("/generate/audio", h.TTS.GenerateAudio)
	protected.Get("/generate/status/:jobId", h.TTS.GetAudioStatus)
	protected.Get("/generate/voices", h.TTS.GetVoices)

	// Dashboard + library (Sprint 6)
	protected.Get("/dashboard/stats",       h.Dashboard.GetStats)
	protected.Get("/dashboard/recent",      h.Dashboard.GetRecent)
	protected.Patch("/dashboard/ads/:id/audio", h.Dashboard.LinkAudio)

	

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
