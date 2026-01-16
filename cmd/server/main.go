package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"seculoc-back/internal/app"
	"seculoc-back/internal/platform/logger"
)

// @title           Seculoc API
// @version         1.0
// @description     Backend API for Seculoc, a rental management platform.
// @termsOfService  http://swagger.io/terms/

// @contact.name    API Support
// @contact.url     http://www.swagger.io/support
// @contact.email   support@swagger.io

// @license.name    Apache 2.0
// @license.url     http://www.apache.org/licenses/LICENSE-2.0.html

// @host            localhost:8080
// @BasePath        /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

func main() {
	// 1. Initialize Configuration
	initConfig()

	// 2. Initialize Logger
	logger.Init(viper.GetString("ENV"))
	log := logger.Get()
	defer logger.Sync()

	log.Info("Starting Seculoc Backend...")

	// 3. Database Connection
	// Fallback/Default handling could vary, but here we expect env vars or .env
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

	// 4. Start Server via App Wiring
	r := app.NewServer(pool, log)

	// 5. Start Server
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
		// Log warning but proceed if relying on system envs
		fmt.Fprintf(os.Stderr, "Config file not found: %s \n", err)
	}

	// Set defaults
	viper.SetDefault("JWT_SECRET", "change_me_in_prod")
	viper.SetDefault("JWT_EXPIRATION_HOURS", 24)
}
