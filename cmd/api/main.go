package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	adminhandler "github.com/cloud-nullus/draft/internal/admin/adapter/handler"
	adminrepo "github.com/cloud-nullus/draft/internal/admin/adapter/repository"
	"github.com/cloud-nullus/draft/internal/admin/usecase"
	logadapter "github.com/cloud-nullus/draft/internal/stack/adapter/log"
	stackhandler "github.com/cloud-nullus/draft/internal/stack/adapter/handler"
	stackrepo "github.com/cloud-nullus/draft/internal/stack/adapter/repository"
	stackuc "github.com/cloud-nullus/draft/internal/stack/usecase"
	"github.com/cloud-nullus/draft/internal/shared/config"
	"github.com/cloud-nullus/draft/internal/shared/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig("configs/config.yaml")
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Initialize database pool
	dsn := fmt.Sprintf("host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.Name,
		cfg.Database.User, cfg.Database.Password, cfg.Database.SSLMode,
	)
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Initialize repositories
	orgRepo := adminrepo.NewPostgresOrgRepository(pool)
	clusterRepo := adminrepo.NewPostgresClusterRepository(pool)

	// Initialize use cases
	orgUC := usecase.NewOrgUseCase(orgRepo)
	clusterUC := usecase.NewClusterUseCase(clusterRepo)

	// Initialize handlers
	orgHandler := adminhandler.NewOrgHandler(orgUC)
	clusterHandler := adminhandler.NewClusterHandler(clusterUC)

	// Stack: in-memory repos + log streamer
	memStackRepo := stackrepo.NewMemoryStackRepository()
	memStreamer := logadapter.NewMemoryStreamer()
	installStackUC := stackuc.NewInstallStack(memStackRepo, memStreamer)
	deployHandler := stackhandler.NewDeployHandler(installStackUC, memStackRepo, memStreamer)

	// Compatibility
	memCompatRepo := stackrepo.NewMemoryCompatibilityRepository()
	validateCompatUC := stackuc.NewValidateCompatibility(memCompatRepo)
	compatHandler := stackhandler.NewCompatibilityHandler(memCompatRepo, validateCompatUC)

	// History
	memHistoryRepo := stackrepo.NewMemoryHistoryRepository()
	manageHistoryUC := stackuc.NewManageHistory(memHistoryRepo)
	historyHandler := stackhandler.NewHistoryHandler(memHistoryRepo, memStackRepo, manageHistoryUC)

	// Initialize Echo
	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = middleware.AppErrorHandler

	// Global middleware
	e.Use(echomw.Recover())
	e.Use(echomw.RequestID())
	e.Use(middleware.SlogLogger())

	// API v1 group
	v1 := e.Group("/api/v1")
	orgHandler.RegisterRoutes(v1)
	clusterHandler.RegisterRoutes(v1)
	deployHandler.RegisterRoutes(v1, e)
	compatHandler.RegisterRoutes(v1)
	historyHandler.RegisterRoutes(v1)

	// Health check
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"status": "ok",
		})
	})

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	go func() {
		slog.Info("starting server", "addr", addr)
		if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.Shutdown(ctx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}
	slog.Info("server stopped")
}
