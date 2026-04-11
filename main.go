// @title Monitor Service API
// @version 1.0
// @description Monitoring service for digi-pulse
// @host localhost:8080
// @BasePath /
// @schemes http https
package main

import (
	"log"
	"net/http"

	"context"
	_ "monitor/api/swagger"
	"monitor/internal/config"
	"monitor/internal/handler"
	"monitor/internal/middleware"
	"monitor/internal/service"
	"monitor/internal/worker"

	"github.com/swaggo/http-swagger/v2"
)

const version = "1.0.0"

func main() {
	cfg := config.Load()

	// Initialize services
	monitorService := service.NewMonitorService(version)
	healthHandler := handler.NewHealthHandler(version)

	// Start workers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	redisWorker := worker.NewWorker(cfg)
	go redisWorker.Start(ctx)

	// Create router
	mux := http.NewServeMux()

	// Register handlers
	mux.HandleFunc("GET /", healthHandler.RootHandler)
	mux.HandleFunc("GET /health", healthHandler.HealthCheck)
	mux.HandleFunc("GET /info", healthHandler.ServiceInfo)

	// Swagger documentation
	mux.Handle("GET /swagger/", httpSwagger.WrapHandler)

	// Apply middleware
	handler := middleware.LoggingMiddleware(mux)

	log.Printf("Server starting on port %s", cfg.Server.Port)
	log.Printf("Swagger UI available at http://localhost:%s/swagger/index.html", cfg.Server.Port)
	log.Printf("Service version: %s", monitorService.GetVersion())

	if err := http.ListenAndServe(":"+cfg.Server.Port, handler); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
