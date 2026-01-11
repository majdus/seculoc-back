package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"seculoc-back/internal/adapter/http/middleware"
	"seculoc-back/internal/platform/logger"
)

func main() {
	// 1. Load Config (Viper)
	viper.SetConfigFile(".env")
	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err != nil {
		// Log but continue (config might be provided via ENV vars in production/docker)
		// We can't use the configured logger yet, so use standard log or panic if critical
		// But let's initialize logger first with a default
	}

	// 2. Initialize Logger
	logger.Init(viper.GetString("LOG_LEVEL"))
	log := logger.Get()
	defer logger.Sync()

	log.Info("Starting Séculoc Backend...")

	// 3. Database Connection
	dbHost := viper.GetString("DB_HOST")
	dbPort := viper.GetString("DB_PORT")
	dbUser := viper.GetString("DB_USER")
	dbPass := viper.GetString("DB_PASSWORD")
	dbName := viper.GetString("DB_NAME")
	dbSSL := viper.GetString("DB_SSL_MODE")

	dbURL := "postgres://" + dbUser + ":" + dbPass + "@" + dbHost + ":" + dbPort + "/" + dbName + "?sslmode=" + dbSSL

	ctx := context.Background()
	poolConfig, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Fatal("Unable to parse database URL", zap.Error(err))
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		log.Fatal("Unable to connect to database", zap.Error(err))
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatal("Database ping failed", zap.Error(err))
	}
	log.Info("Connected to PostgreSQL")

	// 3. Setup HTTP Server (Gin)
	r := gin.New()

	// Middleware
	r.Use(gin.Recovery()) // Recovery middleware recovers from any panics and writes a 500 if there was one.
	r.Use(middleware.RequestLogger())

	// Routes
	r.GET("/health", func(c *gin.Context) {
		// Example of using the injected logger (optional, as middleware logs requests)
		// l := middleware.GetLogger(c.Request.Context())
		// l.Debug("Health check called")
		c.JSON(http.StatusOK, gin.H{
			"status": "up",
			"db":     "connected",
		})
	})

	// 4. Start Server with Graceful Shutdown
	addr := viper.GetString("SERVER_ADDRESS")
	if addr == "" {
		addr = ":8080"
	}
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Listen and serve failed", zap.Error(err))
		}
	}()

	log.Info("Server listening", zap.String("addr", addr))

	// Wait for interrupt signal to gracefully shutdown the server with a timeout of 5 seconds.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown", zap.Error(err))
	}

	log.Info("Server exiting")
}
