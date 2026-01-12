package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"seculoc-back/internal/adapter/http/handler"
	"seculoc-back/internal/adapter/http/middleware"
	"seculoc-back/internal/adapter/storage/postgres"
	"seculoc-back/internal/core/service"
	"seculoc-back/internal/platform/logger"
)

func main() {
	// 1. Initialize Configuration
	initConfig()

	// 2. Initialize Logger
	logger.Init(viper.GetString("ENV"))
	log := logger.Get()
	defer logger.Sync()
	// Zap global is set inside Init/Get usually, but we can ensure it.
	// Actually the logger package handles it.

	log.Info("Starting Seculoc Backend...")

	// 3. Database Connection
	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		viper.GetString("DB_USER"),
		viper.GetString("DB_PASSWORD"),
		viper.GetString("DB_HOST"),
		viper.GetString("DB_PORT"),
		viper.GetString("DB_NAME"),
		viper.GetString("DB_SSL_MODE"),
	)

	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Fatal("Unable to parse database config", zap.Error(err))
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		log.Fatal("Unable to connect to database", zap.Error(err))
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		log.Fatal("Failed to ping database", zap.Error(err))
	}
	log.Info("Database connected successfully")

	// 4. Persistence Layer
	txManager := postgres.NewTxManager(pool)

	// 5. Service Layer
	userService := service.NewUserService(txManager, log)
	propService := service.NewPropertyService(txManager, log)
	subService := service.NewSubscriptionService(txManager, log)
	solvService := service.NewSolvencyService(txManager, log)

	// 6. Adapters (Handlers)
	userHandler := handler.NewUserHandler(userService)
	propHandler := handler.NewPropertyHandler(propService)
	subHandler := handler.NewSubscriptionHandler(subService)
	solvHandler := handler.NewSolvencyHandler(solvService)

	// 7. HTTP Router (Gin)
	if viper.GetString("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()

	// Middleware
	r.Use(middleware.RequestLogger())
	r.Use(gin.Recovery())

	// Public Routes
	api := r.Group("/api/v1")
	{
		authGroup := api.Group("/auth")
		{
			authGroup.POST("/register", userHandler.Register)
			authGroup.POST("/login", userHandler.Login)
		}

		// Protected Routes
		protected := api.Group("/")
		protected.Use(middleware.AuthMiddleware())
		{
			// Properties
			protected.POST("/properties", propHandler.Create)
			protected.GET("/properties", propHandler.List)

			// Subscriptions
			protected.POST("/subscriptions", subHandler.Subscribe)
			protected.POST("/subscriptions/upgrade", subHandler.IncreaseLimit)

			// Solvency
			protected.POST("/solvency/check", solvHandler.CreateCheck)
		}
	}

	// Health Check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 8. Start Server
	addr := viper.GetString("SERVER_ADDRESS")
	if addr == "" {
		addr = ":8080"
	}

	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	log.Info("Server listening", zap.String("address", addr))
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal("Server failed", zap.Error(err))
	}
}

func initConfig() {
	viper.SetConfigFile(".env")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		fmt.Println("No .env file found, relying on environment variables")
	}

	// Set defaults
	viper.SetDefault("JWT_SECRET", "change_me_in_prod")
}
